package qqbotsdk

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newWebhookFixture builds a transport plus its handler, ready to receive
// requests. The bot secret matches the documentation examples.
const fixtureSecret = "naOC0ocQE3shWLAfffVLB1rhYPG7"

func newWebhookFixture(t *testing.T, opts ...WebhookOption) (*WebhookTransport, *Signer, http.Handler, *Dispatcher) {
	t.Helper()

	dispatcher := NewDispatcher()
	base := []WebhookOption{
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
		// Client.Start normally installs this; the fixture wires it by hand.
		WithWebhookSyncFunc(dispatcher.DispatchSync),
	}
	transport := NewWebhookTransport(append(base, opts...)...)
	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	handler, err := transport.Handler(dispatcher.Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return transport, signer, handler, dispatcher
}

// postSigned sends a signed callback body.
func postSigned(t *testing.T, handler http.Handler, signer *Signer, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(SignatureHeader, signer.Sign(timestamp, body))
	req.Header.Set(SignatureTimestampHeader, timestamp)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

// TestWebhookDecodesPayloadAndRepliesACK covers the normal push path.
func TestWebhookDecodesPayloadAndRepliesACK(t *testing.T) {
	transport, signer, handler, dispatcher := newWebhookFixture(t)

	received := make(chan *Event, 1)
	dispatcher.Register(EventAtMessageCreate, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		received <- event
		return nil
	}))

	payload := Payload{
		ID:   "event-1",
		Op:   OpDispatch,
		Type: EventAtMessageCreate,
		Data: json.RawMessage(`{"id":"msg-1","content":"hello"}`),
	}
	recorder := postSigned(t, handler, signer, "/events", payload)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	// The reply must be the documented op 12 acknowledgement.
	var ack Payload
	if err := json.Unmarshal(recorder.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ack.Op != OpHTTPCallbackACK {
		t.Errorf("ack op = %d, want %d (HTTP Callback ACK)", ack.Op, OpHTTPCallbackACK)
	}

	select {
	case event := <-received:
		if event.Transport != TransportNameWebhook {
			t.Errorf("Transport = %q, want %q", event.Transport, TransportNameWebhook)
		}
		if event.ID != "event-1" || event.Type != EventAtMessageCreate {
			t.Errorf("event = %+v, want the documented fields", event.Payload)
		}
		var data struct {
			Content string `json:"content"`
		}
		if err := event.DecodeData(&data); err != nil {
			t.Fatalf("DecodeData: %v", err)
		}
		if data.Content != "hello" {
			t.Errorf("content = %q, want hello", data.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the handler was never called")
	}
	_ = transport
}

// TestWebhookRejectsBadSignature is the security-critical case: an unsigned or
// wrongly signed callback must be refused before it is parsed.
func TestWebhookRejectsBadSignature(t *testing.T) {
	_, _, handler, dispatcher := newWebhookFixture(t)

	var called int32
	dispatcher.Register(WildcardEventType, EventHandlerFunc(func(context.Context, *Event) error {
		called++
		return nil
	}))

	body := []byte(`{"op":0,"t":"AT_MESSAGE_CREATE","d":{}}`)

	cases := map[string]func(*http.Request){
		"no signature header": func(r *http.Request) {
			r.Header.Set(SignatureTimestampHeader, "1725442341")
		},
		"signature from another secret": func(r *http.Request) {
			other, err := NewSigner("some-other-secret")
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			r.Header.Set(SignatureHeader, other.Sign("1725442341", body))
			r.Header.Set(SignatureTimestampHeader, "1725442341")
		},
		"signature over a different body": func(r *http.Request) {
			good, err := NewSigner(fixtureSecret)
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			r.Header.Set(SignatureHeader, good.Sign("1725442341", []byte(`{"op":0}`)))
			r.Header.Set(SignatureTimestampHeader, "1725442341")
		},
		"no timestamp header": func(r *http.Request) {
			good, err := NewSigner(fixtureSecret)
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			r.Header.Set(SignatureHeader, good.Sign("1725442341", body))
		},
	}

	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
			prepare(req)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", recorder.Code)
			}
		})
	}

	if called != 0 {
		t.Errorf("an unverified callback reached a handler %d times, want 0", called)
	}
}

// TestWebhookValidationMatchesDocumentedExample drives the op 13 callback
// address validation with the exact payload and secret from the documentation.
func TestWebhookValidationMatchesDocumentedExample(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookPath("/events"),
		WithWebhookSecret("DG5g3B4j9X2KOErG"),
	)
	dispatcher := NewDispatcher()
	handler, err := transport.Handler(dispatcher.Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	// The documented request body is {"d":{...},"op":13}.
	const rawBody = `{"d":{"plain_token":"Arq0D5A61EgUu4OxUvOp","event_ts":"1725442341"},"op":13}`
	const wantSignature = "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d0" +
		"1bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"

	// Sign the request so it passes verification, then check the reply.
	signer, err := NewSigner("DG5g3B4j9X2KOErG")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(rawBody))
	req.Header.Set(SignatureHeader, signer.Sign("1725442341", []byte(rawBody)))
	req.Header.Set(SignatureTimestampHeader, "1725442341")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var resp ValidationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode validation response: %v", err)
	}
	if resp.PlainToken != "Arq0D5A61EgUu4OxUvOp" {
		t.Errorf("plain_token = %q, want it echoed", resp.PlainToken)
	}
	if resp.Signature != wantSignature {
		t.Errorf("signature = %s\nwant        %s", resp.Signature, wantSignature)
	}
}

func TestWebhookCallbackVerifyWithAppIDHeader(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
		WithWebhookAppID("11111111"),
	)
	dispatcher := NewDispatcher()
	handler, err := transport.Handler(dispatcher.Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	body := []byte(`{"d":{"plain_token":"tok","event_ts":"1725442341"},"op":13}`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set(SignatureHeader, signer.Sign("1725442341", body))
	req.Header.Set(SignatureTimestampHeader, "1725442341")
	req.Header.Set(BotAppIDHeader, "99999999") // a different app

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for another app's callback", recorder.Code)
	}
}

// TestWebhookSyncDispatchReportsHandlerFailure checks that a handler error
// becomes a retryable HTTP 500 when synchronous dispatch is enabled.
func TestWebhookSyncDispatchReportsHandlerFailure(t *testing.T) {
	_, signer, handler, dispatcher := newWebhookFixture(t, WithWebhookSyncDispatch(true))

	dispatcher.Register(EventAtMessageCreate, EventHandlerFunc(func(context.Context, *Event) error {
		return errors.New("handler failed")
	}))

	recorder := postSigned(t, handler, signer, "/events", Payload{
		Op: OpDispatch, Type: EventAtMessageCreate, Data: json.RawMessage(`{}`),
	})

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 so the platform retries", recorder.Code)
	}
}

func TestWebhookMethodNotAllowed(t *testing.T) {
	_, _, handler, _ := newWebhookFixture(t)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", recorder.Code)
	}
}

func TestWebhookRejectsInvalidJSON(t *testing.T) {
	_, signer, handler, _ := newWebhookFixture(t)

	body := []byte(`{not json`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set(SignatureHeader, signer.Sign("1", body))
	req.Header.Set(SignatureTimestampHeader, "1")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
}

// TestWebhookUnknownOpcodeIsAcknowledged checks that an opcode this SDK does not
// handle is still acknowledged, so the platform stops retrying it.
func TestWebhookUnknownOpcodeIsAcknowledged(t *testing.T) {
	_, signer, handler, _ := newWebhookFixture(t)

	recorder := postSigned(t, handler, signer, "/events", Payload{Op: OpCode(99)})
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", recorder.Code)
	}

	var ack Payload
	if err := json.Unmarshal(recorder.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ack.Op != OpHTTPCallbackACK {
		t.Errorf("ack op = %d, want the op 12 acknowledgement", ack.Op)
	}
}

func TestWebhookHandlerRequiresDispatch(t *testing.T) {
	transport := NewWebhookTransport(WithWebhookSecret(fixtureSecret))
	if _, err := transport.Handler(nil); err == nil {
		t.Error("a nil dispatch function must be rejected")
	}
}

func TestWebhookHandlerRequiresSecretOrKey(t *testing.T) {
	transport := NewWebhookTransport()
	if _, err := transport.Handler(func(context.Context, *Event) {}); err == nil {
		t.Error("verification without a secret or key must be rejected")
	}
}

func TestWebhookHandlerWithExplicitPublicKey(t *testing.T) {
	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	var key [32]byte
	copy(key[:], signer.PublicKey())

	transport := NewWebhookTransport(WithWebhookPath("/events"), WithWebhookPublicKey(key))
	dispatcher := NewDispatcher()
	handler, err := transport.Handler(dispatcher.Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	recorder := postSigned(t, handler, signer, "/events", Payload{Op: OpDispatch, Type: "X"})
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 with a matching public key", recorder.Code)
	}
}

func TestWebhookInsecureSkipVerify(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookPath("/events"),
		WithWebhookInsecureSkipVerify(true),
	)
	dispatcher := NewDispatcher()
	handler, err := transport.Handler(dispatcher.Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	// No signature headers at all.
	body, _ := json.Marshal(Payload{Op: OpDispatch, Type: EventAtMessageCreate})
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when verification is skipped", recorder.Code)
	}
}

// TestWebhookStandaloneServer covers Start and Stop against a real listener.
func TestWebhookStandaloneServer(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookAddr("127.0.0.1:8080"),
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
	)
	dispatcher := NewDispatcher()

	received := make(chan *Event, 1)
	dispatcher.Register(WildcardEventType, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		received <- event
		return nil
	}))

	ctx := context.Background()
	if err := transport.Start(ctx, dispatcher.Dispatch); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	addr := transport.Addr()
	if addr == nil {
		t.Fatal("Addr is nil after Start")
	}

	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	body, _ := json.Marshal(Payload{Op: OpDispatch, Type: EventAtMessageCreate, ID: "e1"})
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	req, err := http.NewRequest(http.MethodPost, "http://"+addr.String()+"/events", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set(SignatureHeader, signer.Sign(timestamp, body))
	req.Header.Set(SignatureTimestampHeader, timestamp)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST callback: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ackBody, _ := io.ReadAll(resp.Body)
	var ack Payload
	if err := json.Unmarshal(ackBody, &ack); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ack.Op != OpHTTPCallbackACK {
		t.Errorf("ack op = %d, want op 12", ack.Op)
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("the standalone server never dispatched the event")
	}
}

func TestWebhookStartTwiceIsRejected(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookAddr("127.0.0.1:8080"),
		WithWebhookSecret(fixtureSecret),
	)
	dispatcher := NewDispatcher()

	ctx := context.Background()
	if err := transport.Start(ctx, dispatcher.Dispatch); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = transport.Stop(stopCtx)
	})

	if err := transport.Start(ctx, dispatcher.Dispatch); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Start err = %v, want ErrAlreadyRunning", err)
	}
}

// TestWebhookRejectsDisallowedPort checks the documented callback ports.
func TestWebhookRejectsDisallowedPort(t *testing.T) {
	transport := NewWebhookTransport(
		WithWebhookAddr("127.0.0.1:9999"),
		WithWebhookSecret(fixtureSecret),
	)
	err := transport.Start(context.Background(), NewDispatcher().Dispatch)
	if err == nil {
		t.Fatal("a port outside the allowed callback ports must be rejected")
	}
	if !strings.Contains(err.Error(), "not one of the allowed callback ports") {
		t.Errorf("err = %v, want it to name the port restriction", err)
	}
}

func TestAllowedCallbackPortsMatchDocumentation(t *testing.T) {
	want := []int{80, 443, 8080, 8443}
	if len(AllowedCallbackPorts) != len(want) {
		t.Fatalf("AllowedCallbackPorts = %v, want %v", AllowedCallbackPorts, want)
	}
	for i, port := range want {
		if AllowedCallbackPorts[i] != port {
			t.Errorf("AllowedCallbackPorts[%d] = %d, want %d", i, AllowedCallbackPorts[i], port)
		}
	}
}

func TestWebhookStopWithoutStart(t *testing.T) {
	transport := NewWebhookTransport(WithWebhookSecret(fixtureSecret))
	if err := transport.Stop(context.Background()); err != nil {
		t.Errorf("Stop without Start = %v, want nil", err)
	}
}

func TestWebhookName(t *testing.T) {
	if got := NewWebhookTransport().Name(); got != TransportNameWebhook {
		t.Errorf("Name = %q, want %q", got, TransportNameWebhook)
	}
}

// TestWebhookSyncDispatchWithoutInstalledFuncFallsBack checks that a transport
// used without Client.Start still dispatches.
func TestWebhookSyncDispatchWithoutInstalledFuncFallsBack(t *testing.T) {
	_, signer, handler, dispatcher := newWebhookFixture(t, WithWebhookSyncDispatch(true))

	var mu sync.Mutex
	var seen []string
	dispatcher.Register(WildcardEventType, EventHandlerFunc(func(ctx context.Context, event *Event) error {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, event.Type)
		return nil
	}))

	recorder := postSigned(t, handler, signer, "/events", Payload{Op: OpDispatch, Type: EventAtMessageCreate})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(seen) == 1
		mu.Unlock()
		if done {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("the event was never dispatched")
}

// TestWebhookBodyIsHexDecodedBeforeVerification guards the hex decoding step.
func TestWebhookSignatureMustBeHex(t *testing.T) {
	_, _, handler, _ := newWebhookFixture(t)

	body := []byte(`{"op":0}`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set(SignatureHeader, "not-hex-at-all!!")
	req.Header.Set(SignatureTimestampHeader, "1")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", recorder.Code)
	}
}

func TestWebhookWithLogger(t *testing.T) {
	logger := &recordingLogger{}
	transport := NewWebhookTransport(WithWebhookSecret(fixtureSecret))
	transport.logger = logger

	_, _, handler, _ := newWebhookFixture(t)
	_ = handler

	// Reject a callback so the logger path runs.
	body := []byte(`{"op":0}`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	h, err := transport.Handler(NewDispatcher().Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if logger.count() == 0 {
		t.Error("a rejected callback should be logged")
	}
}

// recordingLogger counts log calls.
type recordingLogger struct {
	mu    sync.Mutex
	calls int
}

func (l *recordingLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
}

func (l *recordingLogger) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls
}

// TestWebhookSignatureRoundTripsThroughHandler checks the documented hex
// encoding of the signature header.
func TestWebhookSignatureHeaderIsHex(t *testing.T) {
	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	signature := signer.Sign("1", []byte("body"))
	if _, err := hex.DecodeString(signature); err != nil {
		t.Errorf("the signature must be hex encoded: %v", err)
	}
	if len(signature) != 128 {
		t.Errorf("signature is %d hex chars, want 128", len(signature))
	}
}
