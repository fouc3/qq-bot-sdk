package qqbotsdk

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// logSink captures slog output so a test can assert the default error handler
// wrote through the configured logger.
type logSink struct {
	mu    sync.Mutex
	lines []string
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, string(p))
	return len(p), nil
}

func (s *logSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.lines)
}

// TestWithLoggerRoutesDefaultErrorHandler checks that a configured logger
// receives reports from the default error handler.
func TestWithLoggerRoutesDefaultErrorHandler(t *testing.T) {
	sink := &logSink{}
	d := NewDispatcher(WithLogger(slog.New(slog.NewTextHandler(sink, nil))))

	d.Register(WildcardEventType, EventHandlerFunc(func(context.Context, *Event) error {
		panic("boom")
	}))
	_ = d.DispatchSync(context.Background(), NewEvent(&Payload{Type: "X"}, TransportNameWebhook))

	if sink.count() == 0 {
		t.Error("the default error handler must write through the configured logger")
	}
}

func TestWebhookErrorHandlerOptionIsUsed(t *testing.T) {
	var called bool
	transport := NewWebhookTransport(
		WithWebhookErrorHandler(func(context.Context, *Event, error) { called = true }),
	)
	transport.reportError(context.Background(), nil, errors.New("x"))

	if !called {
		t.Error("WithWebhookErrorHandler must be used")
	}
}

func TestWebhookPathAccessor(t *testing.T) {
	transport := NewWebhookTransport(WithWebhookPath("/custom"))
	if got := transport.Path(); got != "/custom" {
		t.Errorf("Path = %q, want /custom", got)
	}
}

func TestWebSocketURLAccessor(t *testing.T) {
	transport := NewWebSocketTransport("wss://example.invalid/ws")
	if got := transport.URL(); got != "wss://example.invalid/ws" {
		t.Errorf("URL = %q, want the configured address", got)
	}
}

// TestWebSocketHeartbeatIntervalOption covers the fallback used when the server
// does not state an interval.
func TestWebSocketHeartbeatIntervalOption(t *testing.T) {
	interval := 123 * time.Millisecond
	transport := NewWebSocketTransport("wss://x", WithHeartbeatInterval(interval))
	if transport.heartbeatInterval != interval {
		t.Errorf("heartbeatInterval = %v, want %v", transport.heartbeatInterval, interval)
	}
}

// TestWebSocketHeartbeatFallsBackWhenServerOmitsInterval drives a Hello whose
// body carries no heartbeat_interval, so the configured fallback is used.
func TestWebSocketHeartbeatFallsBackWhenServerOmitsInterval(t *testing.T) {
	h := newGatewayHarness(t, func(t *testing.T, conn *websocket.Conn, harness *gatewayHarness) {
		// Read and record frames (identify, then heartbeats) until the client
		// stops.
		for i := 0; i < 20; i++ {
			if _, ok := harness.record(t, conn); !ok {
				return
			}
			_ = conn.WriteJSON(Payload{Op: OpHeartbeatACK})
		}
	})
	// Replace the harness Hello with one that omits the interval.
	h.setHelloBody(`{}`)

	transport := NewWebSocketTransport(h.wsURL(),
		WithWebSocketToken("QQBot T"),
		WithHeartbeatInterval(50*time.Millisecond),
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

	// A heartbeat must still be sent, using the fallback interval.
	if !waitFor(t, 3*time.Second, func() bool {
		_, ok := h.find(OpHeartbeat)
		return ok
	}) {
		t.Error("the client must fall back to the configured heartbeat interval")
	}
}

func TestCloseCodeStringCoversAllDocumentedCodes(t *testing.T) {
	codes := []CloseCode{
		CloseInvalidOpcode, CloseInvalidPayload, CloseInvalidSessionID, CloseInvalidSeq,
		ClosePayloadTooFast, CloseSessionExpired, CloseInvalidShard, CloseTooManyGuilds,
		CloseInvalidVersion, CloseInvalidIntent, CloseIntentNoPermission,
		CloseInternalErrorLow, CloseInternalErrorHigh, CloseBotOffline, CloseBotBanned,
	}
	for _, code := range codes {
		got := code.String()
		if got == "" || strings.HasPrefix(got, "ClickCode(") {
			t.Errorf("CloseCode(%d).String() = %q, want a documented description", code, got)
		}
	}
}

func TestWebhookServeValidationRejectsBadPayload(t *testing.T) {
	signer, err := NewSigner(fixtureSecret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	transport := NewWebhookTransport(WithWebhookPath("/events"), WithWebhookSecret(fixtureSecret))
	handler, err := transport.Handler(NewDispatcher().Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	// op 13 whose d field cannot be decoded into a validation request.
	body := []byte(`{"op":13,"d":"nope"}`)
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(string(body)))
	req.Header.Set(SignatureHeader, signer.Sign("1", body))
	req.Header.Set(SignatureTimestampHeader, "1")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an undecodable validation payload", recorder.Code)
	}
}

// TestWebhookValidationWithoutSignerIsRejected covers the internal guard.
func TestWebhookValidationWithoutSignerIsRejected(t *testing.T) {
	transport := NewWebhookTransport()
	recorder := httptest.NewRecorder()
	payload := &Payload{Op: OpCallbackVerify, Data: mustJSON(ValidationRequest{PlainToken: "p", EventTs: "1"})}

	transport.serveValidation(recorder, payload, nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when no signer is available", recorder.Code)
	}
}

func TestWriteJSONFallsBackOnEncodeFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	// A channel cannot be marshalled.
	writeJSON(recorder, http.StatusOK, make(chan int))

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the body cannot be encoded", recorder.Code)
	}
}

func TestWaitDoneHonoursContext(t *testing.T) {
	never := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := waitDone(ctx, never); !errors.Is(err, context.Canceled) {
		t.Errorf("waitDone = %v, want context.Canceled", err)
	}

	closed := make(chan struct{})
	close(closed)
	if err := waitDone(context.Background(), closed); err != nil {
		t.Errorf("waitDone on a closed channel = %v, want nil", err)
	}
}

// TestWebhookReportsRejectedCallback checks that a rejected callback reaches the
// configured error handler, because it cannot be reported any other way.
func TestWebhookReportsRejectedCallback(t *testing.T) {
	reported := make(chan error, 1)
	transport := NewWebhookTransport(
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
		WithWebhookErrorHandler(func(ctx context.Context, event *Event, err error) {
			select {
			case reported <- err:
			default:
			}
		}),
	)
	handler, err := transport.Handler(NewDispatcher().Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"op":0}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	select {
	case reportErr := <-reported:
		if reportErr == nil {
			t.Error("the reported error must not be nil")
		}
	case <-time.After(time.Second):
		t.Error("a rejected callback must be reported to the error handler")
	}
}

// TestWebhookWithLoggerOptionReachesLogger checks the logger path.
func TestWebhookWithLoggerOptionReachesLogger(t *testing.T) {
	sink := &logSink{}
	transport := NewWebhookTransport(
		WithWebhookPath("/events"),
		WithWebhookSecret(fixtureSecret),
	)
	transport.logger = slogLogger{sink: sink}

	handler, err := transport.Handler(NewDispatcher().Dispatch)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"op":0}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if sink.count() == 0 {
		t.Error("a rejected callback must reach the logger")
	}
}

// slogLogger adapts a logSink to the Logger interface.
type slogLogger struct{ sink *logSink }

func (l slogLogger) Error(msg string, args ...any) {
	_, _ = io.WriteString(l.sink, msg)
}

// TestWebhookAddrBeforeStart covers the nil-listener path.
func TestWebhookAddrBeforeStart(t *testing.T) {
	transport := NewWebhookTransport(WithWebhookSecret(fixtureSecret))
	if addr := transport.Addr(); addr != nil {
		t.Errorf("Addr before Start = %v, want nil", addr)
	}
}

// TestWebSocketSessionIDBeforeConnect covers the empty-session path.
func TestWebSocketSessionIDBeforeConnect(t *testing.T) {
	transport := NewWebSocketTransport("wss://x", WithWebSocketToken("t"))
	if got := transport.SessionID(); got != "" {
		t.Errorf("SessionID before connect = %q, want empty", got)
	}
}
