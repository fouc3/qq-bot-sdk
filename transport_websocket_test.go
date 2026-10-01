package qqbotsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// gatewayHarness is a fake gateway recording what the client sent.
type gatewayHarness struct {
	server *httptest.Server

	mu       sync.Mutex
	received []Payload
	// rawReceived keeps the verbatim frames, so a heartbeat's bare number body
	// can be inspected before decoding.
	rawReceived []string
	conns       int
	// helloBody overrides the body of the Hello frame. Empty means the default
	// 200ms interval.
	helloBody string
}

// setHelloBody replaces the body of the Hello frame sent on each connection.
func (h *gatewayHarness) setHelloBody(body string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.helloBody = body
}

// newGatewayHarness starts a websocket server running the given script for each
// connection. The script runs after Hello was sent.
func newGatewayHarness(t *testing.T, script func(t *testing.T, conn *websocket.Conn, h *gatewayHarness)) *gatewayHarness {
	t.Helper()

	h := &gatewayHarness{}
	upgrader := websocket.Upgrader{}

	h.server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		h.mu.Lock()
		h.conns++
		h.mu.Unlock()

		// The gateway always opens with Hello.
		h.mu.Lock()
		body := h.helloBody
		h.mu.Unlock()
		if body == "" {
			body = `{"heartbeat_interval":200}`
		}
		hello := Payload{Op: OpHello, Data: json.RawMessage(body)}
		if err := conn.WriteJSON(hello); err != nil {
			return
		}
		script(t, conn, h)
	}))
	t.Cleanup(h.server.Close)
	return h
}

// wsURL converts the test server address to a websocket URL.
func (h *gatewayHarness) wsURL() string {
	return "ws" + strings.TrimPrefix(h.server.URL, "http")
}

// record stores a payload the client sent.
//
// A closed connection is not an error: the test may legitimately stop the
// transport while the script is still reading, so the caller gets ok=false and
// ends its script.
func (h *gatewayHarness) record(t *testing.T, conn *websocket.Conn) (Payload, bool) {
	t.Helper()
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return Payload{}, false
	}
	h.mu.Lock()
	h.rawReceived = append(h.rawReceived, string(raw))
	h.mu.Unlock()

	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Logf("decode client message %s: %v", raw, err)
		return Payload{}, false
	}
	h.mu.Lock()
	h.received = append(h.received, payload)
	h.mu.Unlock()
	return payload, true
}

// mustRecord returns a payload the client sent, failing the test when the
// connection closed first.
func (h *gatewayHarness) mustRecord(t *testing.T, conn *websocket.Conn) Payload {
	t.Helper()
	payload, ok := h.record(t, conn)
	if !ok {
		t.Fatalf("the client closed the connection before sending the expected frame")
	}
	return payload
}

// lastRaw returns the most recent raw frame.
func (h *gatewayHarness) lastRaw() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rawReceived) == 0 {
		return ""
	}
	return h.rawReceived[len(h.rawReceived)-1]
}

// find returns the first payload with the given opcode.
func (h *gatewayHarness) find(op OpCode) (Payload, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, payload := range h.received {
		if payload.Op == op {
			return payload, true
		}
	}
	return Payload{}, false
}

// connCount returns how many connections were accepted.
func (h *gatewayHarness) connCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conns
}

// waitFor polls until condition holds or the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return condition()
}

// TestWebSocketIdentifiesAndSendsHeartbeat covers the documented handshake:
// Hello, then Identify, then heartbeats.
func TestWebSocketIdentifiesAndSendsHeartbeat(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		// Identify must be the first frame the client sends.
		identify := h.mustRecord(t, conn)
		if identify.Op != OpIdentify {
			t.Errorf("first client frame op = %d, want %d (Identify)", identify.Op, OpIdentify)
		}

		var data struct {
			Token   string `json:"token"`
			Intents Intent `json:"intents"`
			Shard   [2]int `json:"shard"`
		}
		if err := identify.DecodeData(&data); err != nil {
			t.Errorf("decode identify: %v", err)
		}
		if data.Token != "QQBot ACCESS_TOKEN" {
			t.Errorf("token = %q, want the QQBot scheme", data.Token)
		}
		if data.Intents != IntentsFor(IntentPublicGuildMessages) {
			t.Errorf("intents = %d, want %d", data.Intents, IntentsFor(IntentPublicGuildMessages))
		}
		if data.Shard != [2]int{0, 1} {
			t.Errorf("shard = %v, want [0 1]", data.Shard)
		}

		// Acknowledge with READY so the client learns its session id.
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{
			"session_id": "session-1",
		})})

		// Then expect at least one heartbeat.
		heartbeat := h.mustRecord(t, conn)
		if heartbeat.Op != OpHeartbeat {
			t.Errorf("second client frame op = %d, want %d (Heartbeat)", heartbeat.Op, OpHeartbeat)
		}
		_ = conn.WriteJSON(Payload{Op: OpHeartbeatACK})

		// Keep the connection open until the test ends.
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithIntents(IntentPublicGuildMessages),
		WithWebSocketToken("QQBot ACCESS_TOKEN"),
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

	if !waitFor(t, 3*time.Second, func() bool { return transport.SessionID() == "session-1" }) {
		t.Fatalf("session id = %q, want session-1", transport.SessionID())
	}

	// The first heartbeat carries null because no event was seen yet.
	if !waitFor(t, 3*time.Second, func() bool {
		_, ok := h.find(OpHeartbeat)
		return ok
	}) {
		t.Fatal("the client never sent a heartbeat")
	}
	if raw := h.lastRaw(); !strings.Contains(raw, `"d":null`) {
		t.Errorf("the first heartbeat frame = %s, want a null sequence number", raw)
	}
}

// TestWebSocketDispatchesEvents checks that a dispatch payload reaches handlers
// with the right type and body, and that the sequence number is tracked.
func TestWebSocketDispatchesEvents(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		_, _ = h.record(t, conn) // identify
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{
			"session_id": "s",
		})})

		seq := int64(7)
		_ = conn.WriteJSON(Payload{
			Op:   OpDispatch,
			Type: EventGroupAtMessageCreate,
			Seq:  &seq,
			ID:   "event-7",
			Data: json.RawMessage(`{"content":"hi","group_openid":"g1"}`),
		})
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(), WithWebSocketToken("QQBot T"))
	dispatcher := NewDispatcher()

	type captured struct {
		event *Event
		body  string
	}
	got := make(chan captured, 1)
	dispatcher.Register(EventGroupAtMessageCreate, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		var data struct {
			Content string `json:"content"`
		}
		_ = event.DecodeData(&data)
		got <- captured{event: event, body: data.Content}
		return nil
	}))

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	select {
	case captured := <-got:
		if captured.event.Type != EventGroupAtMessageCreate {
			t.Errorf("type = %q, want %q", captured.event.Type, EventGroupAtMessageCreate)
		}
		if captured.event.ID != "event-7" {
			t.Errorf("id = %q, want event-7", captured.event.ID)
		}
		if seq, ok := captured.event.Sequence(); !ok || seq != 7 {
			t.Errorf("sequence = %d, %v, want 7, true", seq, ok)
		}
		if captured.event.Transport != TransportNameWebSocket {
			t.Errorf("transport = %q, want %q", captured.event.Transport, TransportNameWebSocket)
		}
		if captured.body != "hi" {
			t.Errorf("body content = %q, want hi", captured.body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the event was never dispatched")
	}
}

// TestWebSocketHeartbeatCarriesLatestSequence covers the documented rule that a
// heartbeat echoes the latest sequence number received.
func TestWebSocketHeartbeatCarriesLatestSequence(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		_, _ = h.record(t, conn) // identify
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": "s"})})

		seq := int64(251)
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: "SOMETHING", Seq: &seq, Data: json.RawMessage(`{}`)})

		// Then read heartbeats until one carries the sequence number.
		for i := 0; i < 5; i++ {
			_, _ = h.record(t, conn)
			_ = conn.WriteJSON(Payload{Op: OpHeartbeatACK})
		}
		<-time.After(time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(), WithWebSocketToken("QQBot T"))
	dispatcher := NewDispatcher()

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if !waitFor(t, 3*time.Second, func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, raw := range h.rawReceived {
			if strings.Contains(raw, `"op":1`) && strings.Contains(raw, `"d":251`) {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no heartbeat carried the latest sequence number; frames: %v", h.rawReceived)
	}
}

// TestWebSocketResumesAfterServerReconnect covers op 7 Reconnect: the client must
// reconnect and resume with its stored session and sequence number.
func TestWebSocketResumesAfterServerReconnect(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		h.mu.Lock()
		attempt := h.conns
		h.mu.Unlock()

		if attempt == 1 {
			_, _ = h.record(t, conn) // identify
			seq := int64(1337)
			_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": "session-xyz"})})
			_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: "X", Seq: &seq, Data: json.RawMessage(`{}`)})
			// Ask the client to reconnect.
			_ = conn.WriteJSON(Payload{Op: OpReconnect})
			time.Sleep(100 * time.Millisecond)
			_ = conn.Close()
			return
		}

		// On the second connection the client must resume, not identify.
		resume := h.mustRecord(t, conn)
		if resume.Op != OpResume {
			t.Errorf("second connection first frame = %d, want %d (Resume)", resume.Op, OpResume)
			return
		}
		var data struct {
			Token     string `json:"token"`
			SessionID string `json:"session_id"`
			Seq       int64  `json:"seq"`
		}
		if err := resume.DecodeData(&data); err != nil {
			t.Errorf("decode resume: %v", err)
		}
		if data.SessionID != "session-xyz" {
			t.Errorf("resume session_id = %q, want session-xyz", data.SessionID)
		}
		if data.Seq != 1337 {
			t.Errorf("resume seq = %d, want 1337", data.Seq)
		}
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventResumed})
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithReconnectDelay(10*time.Millisecond, 100*time.Millisecond),
	)
	dispatcher := NewDispatcher()

	var errs []error
	var mu sync.Mutex
	dispatcher = NewDispatcher(WithErrorHandler(func(ctx context.Context, event *Event, err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}))

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if !waitFor(t, 5*time.Second, func() bool { return h.connCount() >= 2 }) {
		t.Fatalf("connections = %d, want the client to reconnect", h.connCount())
	}
	if !waitFor(t, 5*time.Second, func() bool {
		_, ok := h.find(OpResume)
		return ok
	}) {
		t.Fatal("the client never sent a Resume")
	}
}

// TestWebSocketReconnectsAfterDrop checks that a dropped connection is retried.
func TestWebSocketReconnectsAfterDrop(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		h.mu.Lock()
		attempt := h.conns
		h.mu.Unlock()

		_, _ = h.record(t, conn)
		if attempt == 1 {
			// Drop without a close frame, as a network failure would.
			_ = conn.Close()
			return
		}
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": "second"})})
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithReconnectDelay(10*time.Millisecond, 100*time.Millisecond),
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

	if !waitFor(t, 5*time.Second, func() bool { return transport.SessionID() == "second" }) {
		t.Fatalf("the client did not reconnect and ready up; connections = %d", h.connCount())
	}
}

// TestWebSocketStopsOnFatalCloseCode checks that a fatal code ends the retry
// loop instead of reconnecting forever.
func TestWebSocketStopsOnFatalCloseCode(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		_, _ = h.record(t, conn) // identify
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(int(CloseIntentNoPermission), "intent 无权限"),
			time.Now().Add(time.Second))
		_ = conn.Close()
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithReconnectDelay(10*time.Millisecond, 50*time.Millisecond),
	)

	var mu sync.Mutex
	var errs []error
	dispatcher := NewDispatcher(WithErrorHandler(func(ctx context.Context, event *Event, err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}))

	// Client.Start wires this automatically; hand-wiring passes it explicitly.
	transport.installErrorHandler(dispatcher.ReportError)

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	// The fatal code must stop reconnecting: exactly one connection.
	if !waitFor(t, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(errs) > 0
	}) {
		t.Fatal("the fatal close code was never reported")
	}

	time.Sleep(300 * time.Millisecond) // give a wrong implementation time to retry
	if count := h.connCount(); count != 1 {
		t.Errorf("connections = %d, want 1: a fatal close code must not be retried", count)
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, err := range errs {
		if errors.Is(err, errFatalCloseCode) {
			found = true
		}
	}
	if !found {
		t.Errorf("the reported errors do not mark the code as fatal: %v", errs)
	}
}

// TestWebSocketInvalidSessionClearsSession covers op 9: the session cannot be
// resumed, so the next attempt identifies.
func TestWebSocketInvalidSessionClearsSession(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		h.mu.Lock()
		attempt := h.conns
		h.mu.Unlock()

		if attempt == 1 {
			_, _ = h.record(t, conn) // identify
			_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": "gone"})})
			_ = conn.WriteJSON(Payload{Op: OpInvalidSession})
			time.Sleep(50 * time.Millisecond)
			_ = conn.Close()
			return
		}

		frame := h.mustRecord(t, conn)
		if frame.Op != OpIdentify {
			t.Errorf("after an invalid session the client sent op %d, want %d (Identify)", frame.Op, OpIdentify)
		}
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": "fresh"})})
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithReconnectDelay(10*time.Millisecond, 100*time.Millisecond),
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

	if !waitFor(t, 5*time.Second, func() bool { return transport.SessionID() == "fresh" }) {
		t.Fatalf("session = %q, want fresh: the client must identify after an invalid session", transport.SessionID())
	}
}

func TestWebSocketStartValidation(t *testing.T) {
	dispatcher := NewDispatcher()

	if err := NewWebSocketTransport("").Start(context.Background(), dispatcher.Dispatch); err == nil {
		t.Error("a missing gateway URL must be rejected")
	}
	if err := NewWebSocketTransport("wss://x", WithWebSocketToken("t")).Start(context.Background(), nil); err == nil {
		t.Error("a missing dispatch function must be rejected")
	}
	if err := NewWebSocketTransport("wss://x").Start(context.Background(), dispatcher.Dispatch); err == nil {
		t.Error("a missing token source must be rejected")
	}
}

func TestWebSocketStartTwiceIsRejected(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		_, _ = h.record(t, conn)
		<-time.After(2 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(), WithWebSocketToken("QQBot T"))
	dispatcher := NewDispatcher()

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if err := transport.Start(context.Background(), dispatcher.Dispatch); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start err = %v, want ErrAlreadyRunning", err)
	}
}

func TestWebSocketStopWithoutStart(t *testing.T) {
	transport := NewWebSocketTransport("wss://x", WithWebSocketToken("t"))
	if err := transport.Stop(context.Background()); err != nil {
		t.Errorf("Stop without Start = %v, want nil", err)
	}
}

func TestWebSocketName(t *testing.T) {
	if got := NewWebSocketTransport("wss://x").Name(); got != TransportNameWebSocket {
		t.Errorf("Name = %q, want %q", got, TransportNameWebSocket)
	}
}

// TestCloseCodeClassification pins the documented retry rules per close code.
func TestCloseCodeClassification(t *testing.T) {
	cases := []struct {
		code     CloseCode
		resume   bool
		identify bool
		fatal    bool
	}{
		{CloseInvalidOpcode, false, false, true},
		{CloseInvalidPayload, false, false, true},
		{CloseInvalidSessionID, false, true, false},
		{CloseInvalidSeq, false, true, false},
		{ClosePayloadTooFast, true, true, false},
		{CloseSessionExpired, true, true, false},
		{CloseInvalidShard, false, false, true},
		{CloseTooManyGuilds, false, false, true},
		{CloseInvalidVersion, false, false, true},
		{CloseInvalidIntent, false, false, true},
		{CloseIntentNoPermission, false, false, true},
		{CloseCode(4900), false, true, false},
		{CloseCode(4913), false, true, false},
		{CloseBotOffline, false, false, true},
		{CloseBotBanned, false, false, true},
	}

	for _, tc := range cases {
		if got := tc.code.CanResume(); got != tc.resume {
			t.Errorf("%d CanResume = %v, want %v", tc.code, got, tc.resume)
		}
		if got := tc.code.CanIdentify(); got != tc.identify {
			t.Errorf("%d CanIdentify = %v, want %v", tc.code, got, tc.identify)
		}
		if got := tc.code.Fatal(); got != tc.fatal {
			t.Errorf("%d Fatal = %v, want %v", tc.code, got, tc.fatal)
		}
	}
}

func TestClassifyConnectionError(t *testing.T) {
	fatal := classifyConnectionError(&websocket.CloseError{Code: int(CloseBotBanned), Text: "banned"})
	if !errors.Is(fatal, errFatalCloseCode) {
		t.Errorf("a banned close code must be classified fatal, got %v", fatal)
	}

	invalid := classifyConnectionError(&websocket.CloseError{Code: int(CloseInvalidSessionID)})
	if !errors.Is(invalid, errInvalidSession) {
		t.Errorf("an invalid session id must be classified as needing identify, got %v", invalid)
	}

	plain := errors.New("network gone")
	if got := classifyConnectionError(plain); !errors.Is(got, plain) {
		t.Errorf("a non-close error must pass through, got %v", got)
	}
}

func TestCloseCodeString(t *testing.T) {
	if got := CloseIntentNoPermission.String(); !strings.Contains(got, "intent 无权限") {
		t.Errorf("String() = %q, want the documented wording", got)
	}
	if got := CloseBotBanned.String(); !strings.Contains(got, "封禁") {
		t.Errorf("String() = %q, want the documented wording", got)
	}
	if got := CloseCode(9999).String(); !strings.Contains(got, "9999") {
		t.Errorf("String() = %q, want an unknown code to render its number", got)
	}
}

// TestWebSocketTokenFuncIsCalledPerConnection checks that the token is fetched
// again on reconnect, so an expired token recovers.
func TestWebSocketTokenFuncIsCalledPerConnection(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		frame := h.mustRecord(t, conn)
		var data struct {
			Token string `json:"token"`
		}
		_ = frame.DecodeData(&data)
		_ = conn.WriteJSON(Payload{Op: OpDispatch, Type: EventReady, Data: mustJSON(map[string]any{"session_id": data.Token})})
		<-time.After(time.Second)
	})

	var mu sync.Mutex
	calls := 0
	transport := NewWebSocketTransport(h.wsURL(), WithWebSocketTokenFunc(func(ctx context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return fmt.Sprintf("QQBot token-%d", calls), nil
	}))
	dispatcher := NewDispatcher()

	if err := transport.Start(context.Background(), dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if !waitFor(t, 3*time.Second, func() bool { return transport.SessionID() == "QQBot token-1" }) {
		t.Fatalf("session = %q, want the token the func returned", transport.SessionID())
	}
}

// TestWebSocketContextCancellationStops checks that cancelling the context stops
// the transport, so a caller does not leak a connection.
func TestWebSocketContextCancellationStops(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		_, _ = h.record(t, conn)
		<-time.After(5 * time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(), WithWebSocketToken("QQBot T"))
	dispatcher := NewDispatcher()

	ctx, cancel := context.WithCancel(context.Background())
	if err := transport.Start(ctx, dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !waitFor(t, 3*time.Second, func() bool { return h.connCount() >= 1 }) {
		t.Fatal("the client never connected")
	}

	cancel()
	if !waitFor(t, 3*time.Second, func() bool { return !transport.isStarted() }) {
		t.Error("cancelling the context must stop the transport")
	}
}

// TestWebSocketIdentifyContainsProperties documents the properties object.
func TestWebSocketIdentifyContainsProperties(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, h *gatewayHarness) {
		frame := h.mustRecord(t, conn)
		var data struct {
			Properties IdentifyProperties `json:"properties"`
		}
		if err := frame.DecodeData(&data); err != nil {
			t.Errorf("decode identify: %v", err)
		}
		if data.Properties.OS != "linux" || data.Properties.Browser != "my_library" {
			t.Errorf("properties = %+v, want the configured values", data.Properties)
		}
		<-time.After(time.Second)
	})

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithIdentifyProperties(IdentifyProperties{OS: "linux", Browser: "my_library", Device: "my_library"}),
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
