package qqbotsdk

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// stubTransport records lifecycle calls for the client-level tests.
type stubTransport struct {
	name string

	mu       sync.Mutex
	starts   int
	stops    int
	startErr error
	stopErr  error
}

func (s *stubTransport) Name() string { return s.name }

func (s *stubTransport) Start(ctx context.Context, dispatch DispatchFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	return s.startErr
}

func (s *stubTransport) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stops++
	return s.stopErr
}

func (s *stubTransport) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts, s.stops
}

func TestClientStartRequiresTransport(t *testing.T) {
	client := NewClient("id", "secret")
	if err := client.Start(context.Background()); !errors.Is(err, ErrNoTransport) {
		t.Errorf("Start without transports = %v, want ErrNoTransport", err)
	}
}

func TestClientStartAndStopTransports(t *testing.T) {
	client := NewClient("id", "secret")

	first := &stubTransport{name: "first"}
	second := &stubTransport{name: "second"}
	client.UseTransport(first)
	client.UseTransport(second)

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !client.Running() {
		t.Error("Running must report true after Start")
	}

	if err := client.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if client.Running() {
		t.Error("Running must report false after Stop")
	}

	for _, transport := range []*stubTransport{first, second} {
		starts, stops := transport.counts()
		if starts != 1 || stops != 1 {
			t.Errorf("%s: starts = %d, stops = %d, want 1 each", transport.name, starts, stops)
		}
	}
}

func TestClientStartTwiceIsRejected(t *testing.T) {
	client := NewClient("id", "secret")
	client.UseTransport(&stubTransport{name: "t"})

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := client.Start(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start = %v, want ErrAlreadyRunning", err)
	}
}

// TestClientStartRollsBackOnFailure checks that a transport failing to start
// does not leave the earlier ones running.
func TestClientStartRollsBackOnFailure(t *testing.T) {
	client := NewClient("id", "secret")

	first := &stubTransport{name: "first"}
	failingErr := errors.New("cannot start")
	failing := &stubTransport{name: "failing", startErr: failingErr}
	client.UseTransport(first)
	client.UseTransport(failing)

	err := client.Start(context.Background())
	if err == nil {
		t.Fatal("Start must report the failing transport")
	}
	if !errors.Is(err, failingErr) {
		t.Errorf("err = %v, want it to wrap the transport error", err)
	}

	starts, stops := first.counts()
	if starts != 1 || stops != 1 {
		t.Errorf("the already started transport must be stopped again: starts = %d, stops = %d", starts, stops)
	}
	if client.Running() {
		t.Error("Running must be false after a failed Start")
	}
}

func TestClientStopWithoutStart(t *testing.T) {
	client := NewClient("id", "secret")
	if err := client.Stop(context.Background()); err != nil {
		t.Errorf("Stop without Start = %v, want nil", err)
	}
}

func TestClientUseTransportIgnoresNil(t *testing.T) {
	client := NewClient("id", "secret")
	client.UseTransport(nil)
	if got := len(client.Transports()); got != 0 {
		t.Errorf("Transports = %d, want 0", got)
	}
}

func TestClientTransportsReturnsCopy(t *testing.T) {
	client := NewClient("id", "secret")
	client.UseTransport(&stubTransport{name: "t"})

	got := client.Transports()
	got[0] = nil // mutating the copy must not affect the client
	if client.Transports()[0] == nil {
		t.Error("Transports must return a copy")
	}
}

func TestClientRegisterAndDispatch(t *testing.T) {
	client := NewClient("id", "secret")

	got := make(chan string, 1)
	client.RegisterFunc(EventAtMessageCreate, func(ctx context.Context, event *Event) error {
		got <- event.Type
		return nil
	})

	event := NewEvent(&Payload{Type: EventAtMessageCreate}, TransportNameWebSocket)
	if err := client.Dispatcher().DispatchSync(context.Background(), event); err != nil {
		t.Fatalf("DispatchSync: %v", err)
	}

	select {
	case eventType := <-got:
		if eventType != EventAtMessageCreate {
			t.Errorf("handler saw %q, want %q", eventType, EventAtMessageCreate)
		}
	case <-time.After(time.Second):
		t.Fatal("the handler was never called")
	}
}

func TestClientRegistrationCancel(t *testing.T) {
	client := NewClient("id", "secret")

	var mu sync.Mutex
	calls := 0
	reg := client.Register(WildcardEventType, EventHandlerFunc(func(context.Context, *Event) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return nil
	}))

	_ = client.Dispatcher().DispatchSync(context.Background(), NewEvent(&Payload{Type: "X"}, TransportNameWebhook))
	reg.Cancel()
	_ = client.Dispatcher().DispatchSync(context.Background(), NewEvent(&Payload{Type: "X"}, TransportNameWebhook))

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("calls = %d, want 1: the cancelled handler must not run", calls)
	}
}

func TestWithDispatcherOption(t *testing.T) {
	dispatcher := NewDispatcher()
	client := NewClient("id", "secret", WithDispatcher(dispatcher))

	if client.Dispatcher() != dispatcher {
		t.Error("WithDispatcher must install the given dispatcher")
	}

	// A nil dispatcher must not wipe the default.
	client = NewClient("id", "secret", WithDispatcher(nil))
	if client.Dispatcher() == nil {
		t.Error("a nil dispatcher must be ignored")
	}
}

// TestClientContextCancellationStopsTransports checks that cancelling the
// context given to Start shuts the transports down.
func TestClientContextCancellationStopsTransports(t *testing.T) {
	client := NewClient("id", "secret")
	transport := &stubTransport{name: "t"}
	client.UseTransport(transport)

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cancel()

	if !waitFor(t, 2*time.Second, func() bool {
		starts, stops := transport.counts()
		return starts == 1 && stops == 1
	}) {
		starts, stops := transport.counts()
		t.Errorf("starts = %d, stops = %d; cancelling must stop the transports", starts, stops)
	}
}

// TestClientStartInstallsWebhookWiring checks that Start configures the webhook
// transport, so a caller does not have to, and that a failing handler then
// produces a retryable 500.
func TestClientStartInstallsWebhookWiring(t *testing.T) {
	client := NewClient("id", "secret")

	webhook := NewWebhookTransport(
		WithWebhookAddr("127.0.0.1:0"),
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
		WithWebhookSyncDispatch(true),
	)
	client.UseTransport(webhook)

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Stop(stopCtx)
	})

	if webhook.synchronousDispatch() == nil {
		t.Error("Start must install the synchronous dispatch function")
	}

	client.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		return errors.New("handler failed")
	}))

	addr := webhook.Addr()
	if addr == nil {
		t.Fatal("the webhook server is not listening")
	}

	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	body := mustJSON(Payload{Op: OpDispatch, Type: EventAtMessageCreate, Data: mustJSON(map[string]any{})})
	req, err := http.NewRequest(http.MethodPost, "http://"+addr.String()+"/events", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	timestamp := "1725442341"
	req.Header.Set(SignatureHeader, signer.Sign(timestamp, body))
	req.Header.Set(SignatureTimestampHeader, timestamp)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 from the failing handler", resp.StatusCode)
	}
}

func TestClientStartInstallsWebSocketTokenSource(t *testing.T) {
	// No credentials on purpose: the check must not reach the network.
	client := NewClient("", "")

	socket := NewWebSocketTransport("wss://example.invalid", WithIntents(IntentPublicGuildMessages))
	if socket.tokenFunc != nil {
		t.Fatal("a fresh transport must not have a token function")
	}

	client.UseTransport(socket)
	_ = client.Start(context.Background())
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Stop(stopCtx)
	})

	if socket.tokenFunc == nil {
		t.Fatal("Start must install the client's token source on the websocket transport")
	}
	// The installed source is the client's own credential lookup, so without
	// credentials it reports ErrNoCredentials instead of calling the platform.
	if _, err := socket.tokenFunc(context.Background()); !errors.Is(err, ErrNoCredentials) {
		t.Errorf("token func err = %v, want ErrNoCredentials", err)
	}
}

// TestStartInstallsErrorReporting checks that transport failures reach the
// dispatcher's error handler without per-transport configuration.
func TestStartInstallsErrorReporting(t *testing.T) {
	// No credentials on purpose: the check must not reach the network.
	client := NewClient("", "")

	webhook := NewWebhookTransport(
		WithWebhookAddr("127.0.0.1:0"),
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
	)
	socket := NewWebSocketTransport("wss://example.invalid")
	client.UseTransport(webhook)
	client.UseTransport(socket)
	client.UseTransport(&stubTransport{name: "stub"})

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = client.Stop(stopCtx)

	if webhook.onError == nil {
		t.Error("the webhook transport must receive the dispatcher's error reporter")
	}
	if socket.onError == nil {
		t.Error("the websocket transport must receive the dispatcher's error reporter")
	}
}

// TestInstalledErrorHandlerDoesNotOverrideCaller checks the documented layering:
// an explicit per-transport handler wins over the dispatcher's reporter.
func TestInstalledErrorHandlerDoesNotOverrideCaller(t *testing.T) {
	// No credentials on purpose: the check must not reach the network.
	client := NewClient("", "")

	custom := func(context.Context, *Event, error) {}
	socket := NewWebSocketTransport("wss://example.invalid", WithWebSocketErrorHandler(custom))
	client.UseTransport(socket)

	if err := client.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = client.Stop(stopCtx)

	if socket.onError == nil {
		t.Fatal("the transport must keep an error handler")
	}
	socket.reportError(context.Background(), nil, errors.New("x"))
	// The custom handler swallowed the error; reaching here without a panic is
	// the assertion.
}

func TestTransportNames(t *testing.T) {
	if TransportNameWebhook != "webhook" {
		t.Errorf("TransportNameWebhook = %q", TransportNameWebhook)
	}
	if TransportNameWebSocket != "websocket" {
		t.Errorf("TransportNameWebSocket = %q", TransportNameWebSocket)
	}
}

func TestDispatcherReportError(t *testing.T) {
	var mu sync.Mutex
	var errs []error
	d := NewDispatcher(WithErrorHandler(func(ctx context.Context, event *Event, err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}))

	sentinel := errors.New("connection lost")
	d.ReportError(context.Background(), nil, sentinel)

	mu.Lock()
	defer mu.Unlock()
	if len(errs) != 1 || !errors.Is(errs[0], sentinel) {
		t.Errorf("errs = %v, want the reported sentinel", errs)
	}
}

func TestDispatcherReportErrorWithoutHandler(t *testing.T) {
	// With no handler the report goes to the default logger and must not panic.
	d := NewDispatcher()
	d.ReportError(context.Background(), NewEvent(&Payload{Type: "X"}, TransportNameWebhook), errors.New("x"))
}

func TestGatewayParsesResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gateway":
			_, _ = w.Write([]byte(`{"url":"wss://api.bot.qq.com/websocket/"}`))
		case "/gateway/bot":
			_, _ = w.Write([]byte(`{
				"url":"wss://api.bot.qq.com/websocket",
				"shards":9,
				"session_start_limit":{"total":1000,"remaining":999,"reset_after":14400000,"max_concurrency":1}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})

	gateway, err := client.GetGateway(context.Background())
	if err != nil {
		t.Fatalf("GetGateway: %v", err)
	}
	if gateway.URL != "wss://api.bot.qq.com/websocket/" {
		t.Errorf("URL = %q, want the documented address", gateway.URL)
	}

	bot, err := client.GetGatewayBot(context.Background())
	if err != nil {
		t.Fatalf("GetGatewayBot: %v", err)
	}
	if bot.URL != "wss://api.bot.qq.com/websocket" {
		t.Errorf("URL = %q", bot.URL)
	}
	if bot.Shards != 9 {
		t.Errorf("Shards = %d, want 9", bot.Shards)
	}
	if bot.SessionStartLimit.Total != 1000 || bot.SessionStartLimit.Remaining != 999 {
		t.Errorf("SessionStartLimit = %+v, want the documented fields", bot.SessionStartLimit)
	}
	if bot.SessionStartLimit.ResetAfter != 14400000 || bot.SessionStartLimit.MaxConcurrency != 1 {
		t.Errorf("SessionStartLimit = %+v, want reset_after and max_concurrency", bot.SessionStartLimit)
	}
}

func TestGatewayPropagatesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	if _, err := client.GetGateway(context.Background()); err == nil {
		t.Error("GetGateway must surface an HTTP failure")
	}
	if _, err := client.GetGatewayBot(context.Background()); err == nil {
		t.Error("GetGatewayBot must surface an HTTP failure")
	}
}

// TestShardIDMatchesDocumentedRule checks shard_id = (guild_id >> 22) % num_shards.
func TestShardIDMatchesDocumentedRule(t *testing.T) {
	const guildID = uint64(1) << 22 // shifting by 22 gives 1

	if got := ShardID(guildID, 4); got != 1 {
		t.Errorf("ShardID = %d, want 1", got)
	}
	if got := ShardID(guildID*4, 4); got != 0 {
		t.Errorf("ShardID = %d, want 0 for a multiple of the shard count", got)
	}
	if got := ShardID(1, 4); got != 0 {
		t.Errorf("ShardID = %d, want 0 for a guild below one shard unit", got)
	}
	if got := ShardID(guildID, 0); got != 0 {
		t.Errorf("ShardID with no shards = %d, want 0", got)
	}
}

func TestDefaultShardIsSingleShard(t *testing.T) {
	if DefaultShard != (Shard{ID: 0, Count: 1}) {
		t.Errorf("DefaultShard = %+v, want [0 1]", DefaultShard)
	}
}

// TestWebSocketShardIsSent checks that a non-default shard reaches the gateway.
func TestWebSocketShardIsSent(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		frame := h.mustRecord(t, conn)
		var data struct {
			Shard [2]int `json:"shard"`
		}
		if err := frame.DecodeData(&data); err != nil {
			t.Errorf("decode identify: %v", err)
		}
		if data.Shard != [2]int{2, 4} {
			t.Errorf("shard = %v, want [2 4]", data.Shard)
		}
		<-time.After(time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithShard(Shard{ID: 2, Count: 4}),
	)
	dispatcher := NewDispatcher()

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if !waitFor(t, 3*time.Second, func() bool { return h.connCount() >= 1 }) {
		t.Fatal("the client never connected")
	}
}

// TestTransportStateFinishIsConcurrencySafe covers the double-close panic that
// used to happen when a transport's run loop and Stop raced on finish: probing
// the done channel with select and then closing it is not atomic, so two
// callers could both see it open and the second close panicked.
//
// The workers are released from a barrier so they reach the close decision
// together; without the fix this test panics with "close of closed channel".
func TestTransportStateFinishIsConcurrencySafe(t *testing.T) {
	const rounds, workers = 2000, 8

	for round := 0; round < rounds; round++ {
		var state transportState
		if _, err := state.begin(context.Background()); err != nil {
			t.Fatalf("round %d: begin: %v", round, err)
		}
		done := state.end()

		start := make(chan struct{})
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				state.finish()
			}()
		}
		close(start)
		wg.Wait()

		select {
		case <-done:
		default:
			t.Fatalf("round %d: done must be closed once finish ran", round)
		}
	}
}

// TestTransportStateBeginAfterFinish checks that a transport can be started
// again after it stopped, which the reset of the closed flag must allow.
func TestTransportStateBeginAfterFinish(t *testing.T) {
	var state transportState

	if _, err := state.begin(context.Background()); err != nil {
		t.Fatalf("first begin: %v", err)
	}
	first := state.end()
	state.finish()
	select {
	case <-first:
	default:
		t.Fatal("the first run must be closed")
	}

	if _, err := state.begin(context.Background()); err != nil {
		t.Fatalf("second begin: %v", err)
	}
	second := state.end()
	if second == first {
		t.Fatal("a restart must allocate a fresh done channel")
	}
	state.finish()
	select {
	case <-second:
	default:
		t.Fatal("the second run must be closed")
	}

	// finish after finish must be a no-op rather than a panic.
	state.finish()
}
