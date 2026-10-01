package qqbotsdk

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// collectErrors builds an ErrorHandler recording every report.
func collectErrors() (*[]error, ErrorHandler) {
	var mu sync.Mutex
	var errs []error
	handler := func(ctx context.Context, event *Event, err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	return &errs, handler
}

func TestDispatcherRoutesByEventType(t *testing.T) {
	d := NewDispatcher()

	var got [2]string
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		got[0] = event.Type
		return nil
	}))
	d.Register(EventC2CMessageCreate, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		got[1] = event.Type
		return nil
	}))

	d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))

	if got[0] != EventAtMessageCreate {
		t.Errorf("the matching handler received %q, want %q", got[0], EventAtMessageCreate)
	}
	if got[1] != "" {
		t.Errorf("the non-matching handler must not run, got %q", got[1])
	}
}

func TestDispatcherWildcardReceivesEverything(t *testing.T) {
	d := NewDispatcher()

	var mu sync.Mutex
	seen := map[string]bool{}
	d.Register(WildcardEventType, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		seen[event.Type] = true
		return nil
	}))

	for _, eventType := range []string{EventAtMessageCreate, EventC2CMessageCreate, EventGuildCreate} {
		if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: eventType}, TransportNameWebhook)); err != nil {
			t.Fatalf("DispatchSync(%s): %v", eventType, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Errorf("the wildcard handler saw %v, want all three event types", seen)
	}
}

func TestDispatcherRunsExactAndWildcardTogether(t *testing.T) {
	d := NewDispatcher()

	var exact, wildcard int32
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&exact, 1)
		return nil
	}))
	d.Register(WildcardEventType, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&wildcard, 1)
		return nil
	}))

	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)); err != nil {
		t.Fatalf("DispatchSync: %v", err)
	}
	if exact != 1 || wildcard != 1 {
		t.Errorf("exact = %d, wildcard = %d, want both 1", exact, wildcard)
	}
}

func TestDispatcherNoHandlersIsNotAnError(t *testing.T) {
	d := NewDispatcher()
	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: "NOPE"}, TransportNameWebSocket)); err != nil {
		t.Errorf("dispatch without handlers = %v, want nil", err)
	}
	// Dispatch must not block or panic either.
	d.Dispatch(context.Background(), NewEvent(&Payload{Type: "NOPE"}, TransportNameWebSocket))
	d.Wait()
}

// TestDispatcherIsolatesHandlerErrors checks that one failing handler does not
// stop the others, and that its error surfaces.
func TestDispatcherIsolatesHandlerErrors(t *testing.T) {
	failing := errors.New("handler boom")
	errs, errHandler := collectErrors()
	d := NewDispatcher(WithErrorHandler(errHandler))

	var okRan int32
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		return failing
	}))
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&okRan, 1)
		return nil
	}))

	err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))

	if !errors.Is(err, failing) {
		t.Errorf("DispatchSync err = %v, want it to carry the handler error", err)
	}
	if okRan != 1 {
		t.Error("a failing handler must not stop the other handlers")
	}
	if len(*errs) != 1 {
		t.Errorf("the error handler saw %d reports, want 1", len(*errs))
	}
}

// TestDispatcherIsolatesPanics checks that a panicking handler is contained and
// reported, and that the process survives.
func TestDispatcherIsolatesPanics(t *testing.T) {
	errs, errHandler := collectErrors()
	d := NewDispatcher(WithErrorHandler(errHandler))

	var afterRan int32
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		panic("handler exploded")
	}))
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&afterRan, 1)
		return nil
	}))

	err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))

	if err == nil {
		t.Fatal("a panicking handler must surface an error")
	}
	if afterRan != 1 {
		t.Error("a panic must not stop the other handlers")
	}
	if len(*errs) != 1 {
		t.Fatalf("the error handler saw %d reports, want 1", len(*errs))
	}
	if !contains(errs, "handler exploded") {
		t.Errorf("the report must carry the panic value, got %v", *errs)
	}
}

// contains reports whether any error message contains the substring.
func contains(errs *[]error, substring string) bool {
	for _, err := range *errs {
		if err != nil && strings.Contains(err.Error(), substring) {
			return true
		}
	}
	return false
}

func TestDispatcherPanicInErrorHandlerIsContained(t *testing.T) {
	d := NewDispatcher(WithErrorHandler(func(context.Context, *Event, error) {
		panic("error handler exploded too")
	}))
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		panic("first panic")
	}))

	// Neither panic may escape DispatchSync.
	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)); err == nil {
		t.Error("expected the handler error to surface")
	}
}

// TestDispatcherConcurrencyLimit checks that the limit is respected.
func TestDispatcherConcurrencyLimit(t *testing.T) {
	d := NewDispatcher(WithMaxConcurrency(1))

	var running, maxRunning int32
	var mu sync.Mutex

	handler := EventHandlerFunc(func(context.Context, *Event) error {
		current := atomic.AddInt32(&running, 1)
		mu.Lock()
		if current > maxRunning {
			maxRunning = current
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		return nil
	})

	const handlers = 4
	for i := 0; i < handlers; i++ {
		d.Register(EventAtMessageCreate, handler)
	}

	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)); err != nil {
		t.Fatalf("DispatchSync: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if maxRunning > 1 {
		t.Errorf("max concurrent handlers = %d, want the limit of 1", maxRunning)
	}
}

func TestDispatcherRegisterNilHandler(t *testing.T) {
	d := NewDispatcher()
	if reg := d.Register(EventAtMessageCreate, nil); reg != nil {
		t.Error("registering a nil handler must be a no-op")
	}
	if n := d.Handlers(EventAtMessageCreate); n != 0 {
		t.Errorf("Handlers = %d, want 0", n)
	}
}

func TestDispatcherHandlersCount(t *testing.T) {
	d := NewDispatcher()
	if n := d.Handlers(EventAtMessageCreate); n != 0 {
		t.Errorf("Handlers on an empty dispatcher = %d, want 0", n)
	}
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error { return nil }))
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error { return nil }))
	if n := d.Handlers(EventAtMessageCreate); n != 2 {
		t.Errorf("Handlers = %d, want 2", n)
	}
}

// TestRegistrationCancelRemovesHandler covers dynamic de-registration.
func TestRegistrationCancelRemovesHandler(t *testing.T) {
	d := NewDispatcher()

	var calls int32
	reg := d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}))
	keep := d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}))

	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)); err != nil {
		t.Fatalf("DispatchSync: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 before cancellation", calls)
	}

	reg.Cancel()
	reg.Cancel() // cancelling twice must be safe
	_ = keep

	if err := d.DispatchSync(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)); err != nil {
		t.Fatalf("DispatchSync after cancel: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3: only the surviving handler may run", calls)
	}
	if n := d.Handlers(EventAtMessageCreate); n != 1 {
		t.Errorf("Handlers = %d, want 1 after cancelling one of two", n)
	}
}

func TestRegistrationCancelNilSafe(t *testing.T) {
	var reg *Registration
	reg.Cancel() // must not panic
}

// TestDispatcherConcurrentDispatchIsRaceFree is meaningful under -race.
func TestDispatcherConcurrentDispatchIsRaceFree(t *testing.T) {
	d := NewDispatcher()

	var count int64
	d.Register(WildcardEventType, EventHandlerFunc(func(context.Context, *Event) error {
		atomic.AddInt64(&count, 1)
		return nil
	}))

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Dispatch(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))
		}()
	}
	wg.Wait()
	d.Wait()

	if got := atomic.LoadInt64(&count); got != 32 {
		t.Errorf("handler ran %d times, want 32", got)
	}
}

// TestDispatcherWaitForAsyncHandlers checks that Wait blocks until the
// asynchronously dispatched handlers finished.
func TestDispatcherWaitForAsyncHandlers(t *testing.T) {
	d := NewDispatcher()

	release := make(chan struct{})
	done := make(chan struct{})
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		close(done)
		<-release
		return nil
	}))

	d.Dispatch(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))
	<-done

	waited := make(chan struct{})
	go func() {
		d.Wait()
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("Wait returned while a handler was still running")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after the handler finished")
	}
}

// TestDispatcherDispatchDoesNotBlock checks that Dispatch returns before slow
// handlers finish, which is what keeps a transport read loop responsive.
func TestDispatcherDispatchDoesNotBlock(t *testing.T) {
	d := NewDispatcher()

	release := make(chan struct{})
	started := make(chan struct{})
	d.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		close(started)
		<-release
		return nil
	}))

	returned := make(chan struct{})
	go func() {
		d.Dispatch(context.Background(), NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket))
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Dispatch blocked on a slow handler")
	}

	<-started
	close(release)
	d.Wait()
}

func TestEventHandlerFunc(t *testing.T) {
	var called bool
	var handler EventHandler = EventHandlerFunc(func(ctx context.Context, event *Event) error {
		called = true
		return nil
	})
	if err := handler.Handle(context.Background(), NewEvent(&Payload{}, TransportNameWebhook)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !called {
		t.Error("the adapted function was not called")
	}
}
