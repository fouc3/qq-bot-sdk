package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// WildcardEventType is the registration key that receives every event,
// regardless of its type.
const WildcardEventType = "*"

// defaultMaxConcurrency bounds how many handlers run at the same time.
const defaultMaxConcurrency = 64

// Event is one gateway event handed to a handler.
//
// It embeds the documented Payload, so a handler reads the official fields
// directly (event.Type, event.Op, event.DecodeData(...)), and adds only the
// delivery context that the payload itself does not carry.
type Event struct {
	// Payload is the documented gateway structure, the authoritative source
	// of the event id, opcode, body, sequence number and type.
	*Payload
	// Transport names the delivery path: TransportNameWebhook or
	// TransportNameWebSocket.
	Transport string
	// ReceivedAt is when the SDK received the event.
	ReceivedAt time.Time
}

// NewEvent wraps a payload with its delivery context.
func NewEvent(payload *Payload, transport string) *Event {
	return &Event{
		Payload:    payload,
		Transport:  transport,
		ReceivedAt: time.Now(),
	}
}

// EventHandler processes one event.
//
// Handlers run concurrently and must be safe for concurrent use. Returning an
// error does not stop other handlers; the error is reported to the
// dispatcher's ErrorHandler.
type EventHandler interface {
	Handle(ctx context.Context, event *Event) error
}

// EventHandlerFunc adapts a function to EventHandler.
type EventHandlerFunc func(ctx context.Context, event *Event) error

// Handle implements EventHandler.
func (f EventHandlerFunc) Handle(ctx context.Context, event *Event) error {
	return f(ctx, event)
}

// ErrorHandler receives failures that no caller can return: handler errors and
// panics, and errors raised while decoding an event.
//
// The default writes through slog.
type ErrorHandler func(ctx context.Context, event *Event, err error)

// Registration is the handle of one registered handler. Cancel removes it.
type Registration struct {
	dispatcher *Dispatcher
	eventType  string
	handler    EventHandler
	// index identifies this registration within its event type bucket.
	index int
	once  sync.Once
}

// Cancel removes the handler. It is safe to call more than once.
func (r *Registration) Cancel() {
	if r == nil || r.dispatcher == nil {
		return
	}
	r.once.Do(func() {
		r.dispatcher.remove(r)
	})
}

// Dispatcher routes events to the handlers registered for their type.
//
// A Dispatcher is safe for concurrent use, including registration while events
// are being dispatched.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]*Registration
	// nextIndex numbers registrations so a Cancelled one can be located.
	nextIndex int

	errHandler     ErrorHandler
	logger         *slog.Logger
	maxConcurrency int
	sem            chan struct{}
	wg             sync.WaitGroup
}

// DispatcherOption configures a Dispatcher.
type DispatcherOption func(*Dispatcher)

// WithErrorHandler replaces the error handler.
func WithErrorHandler(h ErrorHandler) DispatcherOption {
	return func(d *Dispatcher) { d.errHandler = h }
}

// WithLogger sets the logger used by the default error handler.
func WithLogger(logger *slog.Logger) DispatcherOption {
	return func(d *Dispatcher) { d.logger = logger }
}

// WithMaxConcurrency bounds how many handlers may run at once. A value below 1
// means unlimited.
func WithMaxConcurrency(n int) DispatcherOption {
	return func(d *Dispatcher) { d.maxConcurrency = n }
}

// NewDispatcher returns an empty Dispatcher.
func NewDispatcher(opts ...DispatcherOption) *Dispatcher {
	d := &Dispatcher{
		handlers:       make(map[string][]*Registration),
		maxConcurrency: defaultMaxConcurrency,
	}
	for _, opt := range opts {
		opt(d)
	}
	d.logger = d.loggerOrDefault()
	if d.errHandler == nil {
		d.errHandler = d.logByDefault
	}
	if d.maxConcurrency > 0 {
		d.sem = make(chan struct{}, d.maxConcurrency)
	}
	return d
}

// loggerOrDefault returns the configured logger, or a process default.
func (d *Dispatcher) loggerOrDefault() *slog.Logger {
	if d.logger != nil {
		return d.logger
	}
	return slog.Default()
}

// logByDefault is the error handler used when none was configured.
func (d *Dispatcher) logByDefault(ctx context.Context, event *Event, err error) {
	logger := d.loggerOrDefault()
	attrs := []any{slog.String("error", err.Error())}
	if event != nil {
		attrs = append(attrs,
			slog.String("event_type", event.Type),
			slog.String("event_id", event.ID),
			slog.String("transport", event.Transport),
		)
	}
	logger.ErrorContext(ctx, "qqbotsdk: event handling failed", attrs...)
}

// Register adds a handler for an event type. Use WildcardEventType to receive
// every event.
//
// Handlers registered for the exact type run before wildcard handlers. Within
// one type, handlers run in registration order but concurrently, so no
// ordering is guaranteed between them.
func (d *Dispatcher) Register(eventType string, handler EventHandler) *Registration {
	if handler == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	reg := &Registration{
		dispatcher: d,
		eventType:  eventType,
		handler:    handler,
		index:      d.nextIndex,
	}
	d.nextIndex++
	d.handlers[eventType] = append(d.handlers[eventType], reg)
	return reg
}

// remove drops a registration from its bucket.
func (d *Dispatcher) remove(reg *Registration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	bucket := d.handlers[reg.eventType]
	for i, candidate := range bucket {
		if candidate == reg {
			d.handlers[reg.eventType] = append(bucket[:i], bucket[i+1:]...)
			break
		}
	}
	if len(d.handlers[reg.eventType]) == 0 {
		delete(d.handlers, reg.eventType)
	}
}

// Handlers returns how many handlers are registered for an event type.
func (d *Dispatcher) Handlers(eventType string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.handlers[eventType])
}

// matching returns the handlers of an event type: the exact matches first,
// then the wildcard ones.
func (d *Dispatcher) matching(eventType string) []EventHandler {
	d.mu.RLock()
	defer d.mu.RUnlock()

	exact := d.handlers[eventType]
	wildcard := d.handlers[WildcardEventType]

	handlers := make([]EventHandler, 0, len(exact)+len(wildcard))
	for _, reg := range exact {
		handlers = append(handlers, reg.handler)
	}
	for _, reg := range wildcard {
		handlers = append(handlers, reg.handler)
	}
	return handlers
}

// Dispatch hands the event to every matching handler and returns immediately,
// without waiting for them to finish.
//
// It is the callback a Transport invokes, so it must not block the transport's
// read loop. Use DispatchSync when the caller needs the handlers' result.
func (d *Dispatcher) Dispatch(ctx context.Context, event *Event) {
	handlers := d.matching(event.Type)
	if len(handlers) == 0 {
		return
	}

	for _, handler := range handlers {
		d.wg.Add(1)
		go func(handler EventHandler) {
			defer d.wg.Done()
			d.acquire()
			defer d.release()
			d.run(ctx, handler, event)
		}(handler)
	}
}

// DispatchSync hands the event to every matching handler and waits for all of
// them, returning their combined errors.
//
// Handlers still run concurrently with each other; only the wait is
// synchronous. It suits callers that must finish work before acknowledging,
// such as a webhook that wants to report handler failure to the platform.
func (d *Dispatcher) DispatchSync(ctx context.Context, event *Event) error {
	handlers := d.matching(event.Type)
	if len(handlers) == 0 {
		return nil
	}

	errs := make([]error, len(handlers))
	var wg sync.WaitGroup
	for i, handler := range handlers {
		wg.Add(1)
		go func(i int, handler EventHandler) {
			defer wg.Done()
			d.acquire()
			defer d.release()
			errs[i] = d.run(ctx, handler, event)
		}(i, handler)
	}
	wg.Wait()
	return errors.Join(errs...)
}

// acquire takes a concurrency slot when a limit is configured.
func (d *Dispatcher) acquire() {
	if d.sem != nil {
		d.sem <- struct{}{}
	}
}

// release returns a concurrency slot.
func (d *Dispatcher) release() {
	if d.sem != nil {
		<-d.sem
	}
}

// run invokes one handler, isolating panics and reporting errors.
//
// A panic in a handler must not take down the process or the transport read
// loop, so it is recovered and reported through the error handler.
func (d *Dispatcher) run(ctx context.Context, handler EventHandler, event *Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("qqbotsdk: handler panicked on %s: %v\n%s",
				event.Type, recovered, debug.Stack())
			d.reportError(ctx, event, err)
		}
	}()

	if err = handler.Handle(ctx, event); err != nil {
		err = fmt.Errorf("qqbotsdk: handler for %s: %w", event.Type, err)
		d.reportError(ctx, event, err)
	}
	return err
}

// reportError sends a failure to the configured error handler.
func (d *Dispatcher) reportError(ctx context.Context, event *Event, err error) {
	if d.errHandler == nil {
		return
	}
	// The error handler is caller-supplied; a panic in it would escape the
	// dispatcher's own goroutine.
	defer func() { _ = recover() }()
	d.errHandler(ctx, event, err)
}

// ReportError delivers a failure that did not come from a handler, such as a
// transport connection problem.
//
// Transports report through it so an application can observe every failure in
// one place instead of configuring each transport separately.
func (d *Dispatcher) ReportError(ctx context.Context, event *Event, err error) {
	d.reportError(ctx, event, err)
}

// Wait blocks until every dispatch started so far has finished.
//
// Transports call it on shutdown so in-flight handlers get a chance to
// complete before the process exits.
func (d *Dispatcher) Wait() {
	d.wg.Wait()
}
