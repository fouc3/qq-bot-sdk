package qqbotsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket defaults.
const (
	// DefaultHeartbeatInterval is used when the server does not state one.
	DefaultHeartbeatInterval = 45 * time.Second
	// DefaultReconnectDelay is the first delay before reconnecting.
	DefaultReconnectDelay = time.Second
	// DefaultMaxReconnectDelay caps the reconnect backoff.
	DefaultMaxReconnectDelay = time.Minute
	// DefaultHandshakeTimeout bounds the websocket handshake.
	DefaultHandshakeTimeout = 15 * time.Second

	// heartbeatSlack is added to the read deadline so a slightly late
	// acknowledgement does not drop the connection.
	heartbeatSlack = 10 * time.Second
)

// Sentinel reasons for leaving the read loop.
var (
	errServerReconnect = errors.New("qqbotsdk: server asked the client to reconnect")
	errInvalidSession  = errors.New("qqbotsdk: invalid session")
)

// IdentifyProperties is the properties object of an identify payload.
//
// The documentation states it currently has no effect and may be left empty.
type IdentifyProperties struct {
	OS      string `json:"$os,omitempty"`
	Browser string `json:"$browser,omitempty"`
	Device  string `json:"$device,omitempty"`
}

// Shard identifies one connection of a sharded deployment.
type Shard struct {
	// ID is this connection's index.
	ID int
	// Count is the total number of shards.
	Count int
}

// DefaultShard is the single-shard configuration: shard [0, 1].
var DefaultShard = Shard{ID: 0, Count: 1}

// WebSocketTransport receives events over a websocket connection.
//
// It owns the whole connection lifecycle: the handshake, authentication,
// heartbeats, resuming a dropped session, and reconnecting with backoff.
type WebSocketTransport struct {
	transportState

	url        string
	intents    Intent
	shard      Shard
	properties IdentifyProperties

	// tokenFunc returns the Authorization header value, e.g.
	// "QQBot ACCESS_TOKEN". Client.Start installs the client's own token
	// source here.
	tokenFunc func(ctx context.Context) (string, error)
	// staticToken is used when no tokenFunc is set.
	staticToken string

	dialer *websocket.Dialer

	heartbeatInterval time.Duration
	reconnectDelay    time.Duration
	maxReconnectDelay time.Duration

	onError ErrorHandler
	logger  Logger

	// session state, guarded by stateMu.
	stateMu   sync.Mutex
	sessionID string
	lastSeq   *int64
	conn      *websocket.Conn
	// writeMu serializes writes: a websocket allows only one writer at a time.
	writeMu sync.Mutex
}

// WebSocketOption configures a WebSocketTransport.
type WebSocketOption func(*WebSocketTransport)

// WithIntents sets the subscribed event categories.
func WithIntents(intents Intent) WebSocketOption {
	return func(w *WebSocketTransport) { w.intents = intents }
}

// WithShard sets the shard of this connection.
func WithShard(shard Shard) WebSocketOption {
	return func(w *WebSocketTransport) { w.shard = shard }
}

// WithIdentifyProperties sets the identify properties.
func WithIdentifyProperties(properties IdentifyProperties) WebSocketOption {
	return func(w *WebSocketTransport) { w.properties = properties }
}

// WithWebSocketToken sets a fixed Authorization header value, e.g.
// "QQBot ACCESS_TOKEN".
func WithWebSocketToken(token string) WebSocketOption {
	return func(w *WebSocketTransport) { w.staticToken = token }
}

// WithWebSocketTokenFunc sets a callback returning the Authorization header
// value, so a token that expires can be refreshed between connections.
func WithWebSocketTokenFunc(fn func(ctx context.Context) (string, error)) WebSocketOption {
	return func(w *WebSocketTransport) { w.tokenFunc = fn }
}

// WithHeartbeatInterval overrides the heartbeat interval used when the server
// does not state one.
func WithHeartbeatInterval(d time.Duration) WebSocketOption {
	return func(w *WebSocketTransport) { w.heartbeatInterval = d }
}

// WithReconnectDelay sets the first reconnect delay and the backoff cap.
func WithReconnectDelay(initial, max time.Duration) WebSocketOption {
	return func(w *WebSocketTransport) {
		w.reconnectDelay = initial
		w.maxReconnectDelay = max
	}
}

// WithWebSocketErrorHandler receives connection and dispatch failures.
func WithWebSocketErrorHandler(h ErrorHandler) WebSocketOption {
	return func(w *WebSocketTransport) { w.onError = h }
}

// NewWebSocketTransport returns a websocket transport for the given URL, which
// comes from GetGateway or GetGatewayBot.
func NewWebSocketTransport(url string, opts ...WebSocketOption) *WebSocketTransport {
	w := &WebSocketTransport{
		url:               url,
		shard:             DefaultShard,
		dialer:            websocket.DefaultDialer,
		heartbeatInterval: DefaultHeartbeatInterval,
		reconnectDelay:    DefaultReconnectDelay,
		maxReconnectDelay: DefaultMaxReconnectDelay,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Name implements Transport.
func (w *WebSocketTransport) Name() string { return TransportNameWebSocket }

// URL returns the gateway address.
func (w *WebSocketTransport) URL() string { return w.url }

// SessionID returns the current session id, if the connection was accepted.
func (w *WebSocketTransport) SessionID() string {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	return w.sessionID
}

// Start connects and keeps the connection alive until the context is cancelled
// or Stop is called.
//
// It returns once the connection loop is running; receiving continues in the
// background.
func (w *WebSocketTransport) Start(ctx context.Context, dispatch DispatchFunc) error {
	if dispatch == nil {
		return errors.New("qqbotsdk: websocket transport needs a dispatch function")
	}
	if w.url == "" {
		return errors.New("qqbotsdk: websocket transport needs a gateway URL")
	}
	if w.tokenFunc == nil && w.staticToken == "" {
		return errors.New("qqbotsdk: websocket transport needs a token source")
	}

	runCtx, err := w.begin(ctx)
	if err != nil {
		return err
	}
	go w.run(runCtx, dispatch)
	return nil
}

// Stop closes the connection and waits for the loop to end.
func (w *WebSocketTransport) Stop(ctx context.Context) error {
	if !w.isStarted() {
		return nil
	}
	done := w.end()
	w.closeConn()
	w.finish()
	return waitDone(ctx, done)
}

// run keeps reconnecting until the context ends.
func (w *WebSocketTransport) run(ctx context.Context, dispatch DispatchFunc) {
	defer w.finish()

	delay := w.reconnectDelay
	for {
		if ctx.Err() != nil {
			return
		}

		err := w.connectAndRead(ctx, dispatch)
		if ctx.Err() != nil {
			return
		}

		// A fatal close code means every reconnect would be rejected too, so
		// the loop stops instead of retrying forever.
		if errors.Is(err, errFatalCloseCode) {
			w.reportError(ctx, nil, err)
			return
		}

		switch {
		case errors.Is(err, errInvalidSession):
			// The session cannot be resumed; the next attempt identifies.
			w.clearSession()
			delay = w.reconnectDelay
		case errors.Is(err, errServerReconnect):
			w.reportError(ctx, nil, err)
			delay = w.reconnectDelay
		case err != nil:
			w.reportError(ctx, nil, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		// Back off so a persistent failure does not hammer the gateway.
		if next := delay * 2; next <= w.maxReconnectDelay {
			delay = next
		}
	}
}

// connectAndRead performs one connection: handshake, authentication, then
// reading until the connection fails.
func (w *WebSocketTransport) connectAndRead(ctx context.Context, dispatch DispatchFunc) error {
	dialer := w.dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, _, err := dialer.DialContext(ctx, w.url, http.Header{})
	if err != nil {
		return fmt.Errorf("qqbotsdk: websocket dial %s: %w", w.url, err)
	}
	defer func() {
		w.clearConn(conn)
		_ = conn.Close()
	}()
	w.setConn(conn)

	// The gateway sends Hello first, carrying the heartbeat interval.
	interval, err := w.readHello(conn)
	if err != nil {
		return err
	}

	token, err := w.token(ctx)
	if err != nil {
		return err
	}
	if err := w.authenticate(ctx, conn, token); err != nil {
		return err
	}

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go w.heartbeatLoop(connCtx, conn, interval)

	return w.readLoop(ctx, conn, interval, dispatch)
}

// readHello waits for the Hello payload and returns the heartbeat interval.
func (w *WebSocketTransport) readHello(conn *websocket.Conn) (time.Duration, error) {
	_ = conn.SetReadDeadline(time.Now().Add(DefaultHandshakeTimeout))

	var hello Payload
	if err := conn.ReadJSON(&hello); err != nil {
		return 0, fmt.Errorf("qqbotsdk: websocket read hello: %w", err)
	}
	if hello.Op != OpHello {
		return 0, fmt.Errorf("qqbotsdk: websocket expected Hello (op %d), got %s", OpHello, hello.Op)
	}

	var data struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := hello.DecodeData(&data); err != nil {
		return 0, err
	}
	interval := time.Duration(data.HeartbeatInterval) * time.Millisecond
	if interval <= 0 {
		interval = w.heartbeatInterval
	}
	return interval, nil
}

// authenticate sends Identify, or Resume when a session can be restored.
func (w *WebSocketTransport) authenticate(ctx context.Context, conn *websocket.Conn, token string) error {
	w.stateMu.Lock()
	sessionID := w.sessionID
	seq := w.lastSeq
	w.stateMu.Unlock()

	// Resume continues a dropped session and lets the gateway replay the
	// events missed meanwhile, so it is preferred whenever possible.
	if sessionID != "" && seq != nil {
		payload := Payload{
			Op: OpResume,
			Data: mustJSON(struct {
				Token     string `json:"token"`
				SessionID string `json:"session_id"`
				Seq       int64  `json:"seq"`
			}{Token: token, SessionID: sessionID, Seq: *seq}),
		}
		if err := w.writePayload(conn, payload); err != nil {
			return fmt.Errorf("qqbotsdk: websocket resume: %w", err)
		}
		return nil
	}

	identify := Payload{
		Op: OpIdentify,
		Data: mustJSON(struct {
			Token      string             `json:"token"`
			Intents    Intent             `json:"intents"`
			Shard      [2]int             `json:"shard"`
			Properties IdentifyProperties `json:"properties"`
		}{
			Token:      token,
			Intents:    w.intents,
			Shard:      [2]int{w.shard.ID, w.shard.Count},
			Properties: w.properties,
		}),
	}
	if err := w.writePayload(conn, identify); err != nil {
		return fmt.Errorf("qqbotsdk: websocket identify: %w", err)
	}
	return nil
}

// readLoop reads payloads until the connection fails or the context ends.
func (w *WebSocketTransport) readLoop(ctx context.Context, conn *websocket.Conn, interval time.Duration, dispatch DispatchFunc) error {
	// A read deadline is refreshed before every read, but cancelling the
	// context must interrupt a read that is already blocked. Closing the
	// connection from the watchdog is what unblocks it, because a cancelled
	// context alone does not interrupt a blocked socket read.
	stopWatchdog := make(chan struct{})
	defer close(stopWatchdog)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopWatchdog:
		}
	}()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A connection that stops delivering heartbeats is considered dead.
		_ = conn.SetReadDeadline(time.Now().Add(2*interval + heartbeatSlack))

		var payload Payload
		if err := conn.ReadJSON(&payload); err != nil {
			return classifyConnectionError(err)
		}
		if err := w.handlePayload(ctx, &payload, dispatch); err != nil {
			return err
		}
	}
}

// handlePayload reacts to one received payload.
func (w *WebSocketTransport) handlePayload(ctx context.Context, payload *Payload, dispatch DispatchFunc) error {
	if seq, ok := payload.Sequence(); ok {
		w.setSeq(seq)
	}

	switch payload.Op {
	case OpDispatch:
		switch payload.Type {
		case EventReady:
			w.captureSession(payload)
		case EventResumed:
			// The session is live again; nothing else to do.
		}
		dispatch(ctx, NewEvent(payload, TransportNameWebSocket))
		return nil

	case OpHeartbeatACK:
		return nil

	case OpReconnect:
		// The gateway is asking the client to reconnect and resume.
		return errServerReconnect

	case OpInvalidSession:
		return errInvalidSession

	default:
		// Hello, heartbeats and unknown opcodes need no action here.
		return nil
	}
}

// heartbeatLoop sends a heartbeat every interval with the latest sequence
// number, and reports a missing heartbeat window.
func (w *WebSocketTransport) heartbeatLoop(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// The first heartbeat carries null, because no event was seen yet.
		w.stateMu.Lock()
		seq := w.lastSeq
		w.stateMu.Unlock()

		payload := Payload{Op: OpHeartbeat}
		if seq != nil {
			payload.Data = mustJSON(*seq)
		} else {
			payload.Data = json.RawMessage("null")
		}

		if err := w.writePayload(conn, payload); err != nil {
			w.reportError(ctx, nil, fmt.Errorf("qqbotsdk: websocket heartbeat: %w", err))
			return
		}
	}
}

// token returns the Authorization header value.
func (w *WebSocketTransport) token(ctx context.Context) (string, error) {
	if w.tokenFunc != nil {
		return w.tokenFunc(ctx)
	}
	return w.staticToken, nil
}

// writePayload marshals and writes one payload.
//
// Writes are serialized because the identify, resume and heartbeat paths can
// write concurrently, and a websocket connection permits only one writer.
func (w *WebSocketTransport) writePayload(conn *websocket.Conn, payload Payload) error {
	var data []byte
	var err error
	switch payload.Op {
	case OpHeartbeat:
		// The heartbeat body is a bare sequence number, not an object.
		data, err = marshalHeartbeat(payload)
	default:
		data, err = json.Marshal(payload)
	}
	if err != nil {
		return err
	}

	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

// marshalHeartbeat renders the heartbeat payload, whose d field is a bare
// number or null rather than an object.
func marshalHeartbeat(payload Payload) ([]byte, error) {
	var b []byte
	b = append(b, `{"op":1,"d":`...)
	if len(payload.Data) == 0 {
		b = append(b, "null"...)
	} else {
		b = append(b, payload.Data...)
	}
	b = append(b, '}')
	return b, nil
}

// captureSession stores the session id and shard from a READY payload.
func (w *WebSocketTransport) captureSession(payload *Payload) {
	var data struct {
		SessionID string `json:"session_id"`
	}
	if err := payload.DecodeData(&data); err != nil || data.SessionID == "" {
		return
	}
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.sessionID = data.SessionID
}

// clearSession forgets the session so the next connection identifies instead of
// resuming.
func (w *WebSocketTransport) clearSession() {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.sessionID = ""
	w.lastSeq = nil
}

// setSeq records the latest sequence number.
func (w *WebSocketTransport) setSeq(seq int64) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	value := seq
	w.lastSeq = &value
}

// setConn records the live connection.
func (w *WebSocketTransport) setConn(conn *websocket.Conn) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	w.conn = conn
}

// clearConn forgets the connection when it is the current one.
func (w *WebSocketTransport) clearConn(conn *websocket.Conn) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	if w.conn == conn {
		w.conn = nil
	}
}

// closeConn closes the live connection, if any.
func (w *WebSocketTransport) closeConn() {
	w.stateMu.Lock()
	conn := w.conn
	w.conn = nil
	w.stateMu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
}

// reportError forwards a failure to the configured error handler.
func (w *WebSocketTransport) reportError(ctx context.Context, event *Event, err error) {
	w.mu.Lock()
	onError := w.onError
	logger := w.logger
	w.mu.Unlock()

	if onError != nil {
		defer func() { _ = recover() }()
		onError(ctx, event, err)
		return
	}
	if logger != nil {
		logger.Error("qqbotsdk: websocket transport error", "error", err)
	}
}

// installErrorHandler sets the error handler when the caller did not.
//
// Client.Start calls it before the transport starts; a caller wiring the
// transport by hand uses WithWebSocketErrorHandler.
func (w *WebSocketTransport) installErrorHandler(h ErrorHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.onError == nil {
		w.onError = h
	}
}

// CloseCode is a websocket close code sent by the gateway.
type CloseCode int

// Gateway close codes from the documentation.
const (
	CloseInvalidOpcode      CloseCode = 4001
	CloseInvalidPayload     CloseCode = 4002
	CloseInvalidSeq         CloseCode = 4007
	CloseInvalidSessionID   CloseCode = 4006
	ClosePayloadTooFast     CloseCode = 4008
	CloseSessionExpired     CloseCode = 4009
	CloseInvalidShard       CloseCode = 4010
	CloseTooManyGuilds      CloseCode = 4011
	CloseInvalidVersion     CloseCode = 4012
	CloseInvalidIntent      CloseCode = 4013
	CloseIntentNoPermission CloseCode = 4014
	CloseInternalErrorLow   CloseCode = 4900
	CloseInternalErrorHigh  CloseCode = 4913
	CloseBotOffline         CloseCode = 4914
	CloseBotBanned          CloseCode = 4915
)

// String returns the documented meaning of the close code.
func (c CloseCode) String() string {
	switch {
	case c == CloseInvalidOpcode:
		return "无效的 opcode"
	case c == CloseInvalidPayload:
		return "无效的 payload"
	case c == CloseInvalidSessionID:
		return "无效的 session id，无法继续 resume，请 identify"
	case c == CloseInvalidSeq:
		return "seq 错误"
	case c == ClosePayloadTooFast:
		return "发送 payload 过快，请重新连接，并遵守连接后返回的频控信息"
	case c == CloseSessionExpired:
		return "连接过期，请重连并执行 resume 进行重新连接"
	case c == CloseInvalidShard:
		return "无效的 shard"
	case c == CloseTooManyGuilds:
		return "连接需要处理的 guild 过多，请进行合理的分片"
	case c == CloseInvalidVersion:
		return "无效的 version"
	case c == CloseInvalidIntent:
		return "无效的 intent"
	case c == CloseIntentNoPermission:
		return "intent 无权限"
	case c >= CloseInternalErrorLow && c <= CloseInternalErrorHigh:
		return "内部错误，请重连"
	case c == CloseBotOffline:
		return "机器人已下架，只允许连接沙箱环境"
	case c == CloseBotBanned:
		return "机器人已封禁，不允许连接"
	}
	return fmt.Sprintf("ClickCode(%d)", int(c))
}

// CanResume reports whether the connection may be resumed after this code.
func (c CloseCode) CanResume() bool {
	return c == ClosePayloadTooFast || c == CloseSessionExpired
}

// CanIdentify reports whether a fresh identify may be attempted after this code.
//
// A fatal code such as an unauthorized intent or a banned bot must not be
// retried: reconnecting would just be rejected again.
func (c CloseCode) CanIdentify() bool {
	switch c {
	case CloseInvalidSeq, CloseInvalidSessionID, ClosePayloadTooFast, CloseSessionExpired:
		return true
	}
	return c >= CloseInternalErrorLow && c <= CloseInternalErrorHigh
}

// Fatal reports whether the code means the connection must not be retried.
func (c CloseCode) Fatal() bool {
	switch c {
	case CloseInvalidOpcode, CloseInvalidPayload, CloseInvalidShard,
		CloseTooManyGuilds, CloseInvalidVersion, CloseInvalidIntent,
		CloseIntentNoPermission, CloseBotOffline, CloseBotBanned:
		return true
	}
	return false
}

// closeCodeError wraps a gateway close code.
type closeCodeError struct {
	Code CloseCode
	Text string
}

func (e *closeCodeError) Error() string {
	return fmt.Sprintf("qqbotsdk: websocket closed with code %d (%s)", int(e.Code), e.Code)
}

// classifyConnectionError turns a read failure into a reason the reconnect loop
// understands, and marks a fatal close code so it is not retried forever.
func classifyConnectionError(err error) error {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return err
	}

	code := CloseCode(closeErr.Code)
	wrapped := &closeCodeError{Code: code, Text: closeErr.Text}

	// A fatal code means retrying is pointless, so it is reported as such.
	if code.Fatal() {
		return fmt.Errorf("%w: %w", errFatalCloseCode, wrapped)
	}
	// Session-level failures need a fresh identify, not a resume.
	if code == CloseInvalidSessionID || code == CloseInvalidSeq {
		return fmt.Errorf("%w: %w", errInvalidSession, wrapped)
	}
	return wrapped
}

// errFatalCloseCode marks a close code that must not be retried.
var errFatalCloseCode = errors.New("qqbotsdk: fatal websocket close code")

// marshalJSONOrPanic is deliberately avoided elsewhere; mustJSON exists only
// for payloads built from SDK-owned types that cannot fail to encode.
func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
