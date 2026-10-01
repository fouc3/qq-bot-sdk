package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Transport names, reported on every Event.
const (
	TransportNameWebhook   = "webhook"
	TransportNameWebSocket = "websocket"
)

// DispatchFunc receives a decoded event from a transport.
//
// Transports call it for every event they receive. It must not block the
// transport's read loop; Dispatcher.Dispatch returns immediately for that
// reason.
type DispatchFunc func(ctx context.Context, event *Event)

// Transport delivers gateway events from the platform to the SDK.
//
// Webhook and WebSocket are the two implementations; a caller can supply its
// own, for example to read events from a queue or a test fixture.
type Transport interface {
	// Name identifies the transport on every Event it delivers.
	Name() string
	// Start begins receiving events and reports each one to dispatch.
	//
	// It returns once the transport is running, not when it stops. The
	// transport must stop when ctx is cancelled.
	Start(ctx context.Context, dispatch DispatchFunc) error
	// Stop shuts the transport down and releases its resources.
	Stop(ctx context.Context) error
}

// Dispatcher returns the dispatcher events are routed through.
func (c *Client) Dispatcher() *Dispatcher {
	return c.dispatcher
}

// Register adds an event handler.
//
// Registering for WildcardEventType receives every event. The returned
// Registration can be cancelled to remove the handler again.
func (c *Client) Register(eventType string, handler EventHandler) *Registration {
	return c.dispatcher.Register(eventType, handler)
}

// RegisterFunc adds a function as an event handler.
func (c *Client) RegisterFunc(eventType string, handler func(ctx context.Context, event *Event) error) *Registration {
	return c.dispatcher.Register(eventType, EventHandlerFunc(handler))
}

// UseTransport adds a transport to be started by Start.
func (c *Client) UseTransport(t Transport) {
	if t == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.transports = append(c.transports, t)
}

// Transports returns the configured transports.
func (c *Client) Transports() []Transport {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Transport(nil), c.transports...)
}

// Start runs every configured transport.
//
// Transports start in the order they were added. When one fails to start, the
// ones already started are stopped again, so a partial start leaves nothing
// running. It returns as soon as the transports are up; receiving continues in
// the background until Stop is called or ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	transports := append([]Transport(nil), c.transports...)
	if len(transports) == 0 {
		c.mu.Unlock()
		return ErrNoTransport
	}
	if c.running {
		c.mu.Unlock()
		return ErrAlreadyRunning
	}
	c.running = true
	c.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.runCancel = cancel
	c.mu.Unlock()

	started := make([]Transport, 0, len(transports))
	for _, transport := range transports {
		// A webhook may be configured to acknowledge only after the handlers
		// succeeded, which needs the dispatcher's synchronous entry point.
		if webhook, ok := transport.(*WebhookTransport); ok {
			webhook.SetSyncDispatch(c.dispatcher.DispatchSync)
			// Transport-level failures reach the dispatcher's error handler.
			webhook.installErrorHandler(c.dispatcher.ReportError)
		}
		// The websocket transport authenticates with the client's own token,
		// refreshed on every reconnect, so an expired token recovers by itself.
		if socket, ok := transport.(*WebSocketTransport); ok {
			if socket.tokenFunc == nil {
				socket.tokenFunc = c.authorization
			}
			socket.installErrorHandler(c.dispatcher.ReportError)
		}
		if err := transport.Start(runCtx, c.dispatcher.Dispatch); err != nil {
			for i := len(started) - 1; i >= 0; i-- {
				_ = started[i].Stop(context.WithoutCancel(ctx))
			}
			cancel()
			c.mu.Lock()
			c.running = false
			c.runCancel = nil
			c.mu.Unlock()
			return fmt.Errorf("qqbotsdk: start %s transport: %w", transport.Name(), err)
		}
		started = append(started, transport)
	}

	// Cancelling the caller's context must shut the transports down, so the
	// process does not keep receiving events after the caller gave up.
	go func() {
		<-runCtx.Done()
		_ = c.Stop(context.WithoutCancel(ctx))
	}()
	return nil
}

// Stop shuts every transport down in reverse order and waits for the handlers
// that are still running.
//
// It is safe to call more than once, and safe to call without a preceding
// Start.
func (c *Client) Stop(ctx context.Context) error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	cancel := c.runCancel
	c.runCancel = nil
	transports := append([]Transport(nil), c.transports...)
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var errs []error
	for i := len(transports) - 1; i >= 0; i-- {
		if err := transports[i].Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", transports[i].Name(), err))
		}
	}

	// Let in-flight handlers finish before reporting shutdown complete.
	c.dispatcher.Wait()
	return errors.Join(errs...)
}

// Running reports whether Start has been called and Stop has not.
func (c *Client) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// transportState is embedded in the concrete transports to share their
// lifecycle bookkeeping.
type transportState struct {
	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

// begin marks the transport as started and returns a context that stops when
// either the caller's context is cancelled or stop is called.
func (s *transportState) begin(ctx context.Context) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil, ErrAlreadyRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.started = true
	s.cancel = cancel
	s.done = make(chan struct{})
	return runCtx, nil
}

// end marks the transport as stopped and returns the channel closed when it
// finishes.
func (s *transportState) end() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

// finish cancels the run context and closes the done channel.
func (s *transportState) finish() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.started = false
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
			// Already closed by a concurrent stop.
		default:
			close(done)
		}
	}
}

// isStarted reports whether the transport is running.
func (s *transportState) isStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// waitDone blocks until the transport stops or ctx is done.
func waitDone(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
