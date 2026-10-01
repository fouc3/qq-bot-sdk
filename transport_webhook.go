package qqbotsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// maxWebhookBodyBytes caps how much of a callback body is read.
const maxWebhookBodyBytes = 1 << 20

// Webhook defaults.
const (
	// DefaultWebhookPath is where callbacks are received when none is set.
	DefaultWebhookPath = "/"
	// DefaultWebhookAddr listens on every interface on port 8080, one of the
	// four ports the platform is allowed to call back on.
	DefaultWebhookAddr = ":8080"

	defaultReadHeaderTimeout = 10 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
)

// AllowedCallbackPorts lists the ports the platform may call back on.
var AllowedCallbackPorts = []int{80, 443, 8080, 8443}

// WebhookTransport receives events pushed over HTTP callbacks.
//
// It can run its own HTTP server through Start, or be mounted on an existing
// server with Handler. The latter is usually what a bot embedded in a larger
// service wants.
type WebhookTransport struct {
	transportState

	addr   string
	path   string
	secret string
	appID  string

	// publicKey verifies signatures when it is set, taking precedence over the
	// secret.
	publicKey [32]byte
	hasKey    bool

	// allowUnverified disables signature verification. It exists only for
	// tests and for deployments where a proxy already authenticates callbacks.
	allowUnverified bool

	// syncDispatch makes the handler wait for the handlers, so their failure
	// can be reported to the platform as a retryable error.
	syncDispatch bool
	// syncDispatchFunc runs the event synchronously when syncDispatch is set.
	// Client.Start installs the dispatcher's synchronous entry point here.
	syncDispatchFunc func(context.Context, *Event) error

	// onError receives transport-level failures and asynchronous dispatch
	// failures. Client.Start installs the dispatcher's reporter when none was
	// configured.
	onError ErrorHandler
	logger  Logger

	server   *http.Server
	listener net.Listener
}

// Logger is the minimal logging surface the transports use, so a caller can
// plug in any logger.
type Logger interface {
	Error(msg string, args ...any)
}

// WebhookOption configures a WebhookTransport.
type WebhookOption func(*WebhookTransport)

// WithWebhookAddr sets the listen address of the standalone server.
func WithWebhookAddr(addr string) WebhookOption {
	return func(w *WebhookTransport) { w.addr = addr }
}

// WithWebhookPath sets the callback path.
func WithWebhookPath(path string) WebhookOption {
	return func(w *WebhookTransport) { w.path = path }
}

// WithWebhookSecret sets the bot secret used to verify callback signatures.
func WithWebhookSecret(secret string) WebhookOption {
	return func(w *WebhookTransport) { w.secret = secret }
}

// WithWebhookPublicKey verifies signatures with an explicit public key instead
// of deriving one from the secret.
func WithWebhookPublicKey(key [32]byte) WebhookOption {
	return func(w *WebhookTransport) {
		w.publicKey = key
		w.hasKey = true
	}
}

// WithWebhookAppID rejects callbacks whose X-Bot-Appid does not match.
func WithWebhookAppID(appID string) WebhookOption {
	return func(w *WebhookTransport) { w.appID = appID }
}

// WithWebhookSyncDispatch makes the handler wait for the event handlers before
// replying, so a handler failure becomes an HTTP 500 the platform retries.
func WithWebhookSyncDispatch(sync bool) WebhookOption {
	return func(w *WebhookTransport) { w.syncDispatch = sync }
}

// WithWebhookSyncFunc supplies the function used to run handlers synchronously.
//
// Client.Start wires the dispatcher's own entry point automatically, so this is
// only needed when a transport is driven by hand.
func WithWebhookSyncFunc(fn func(context.Context, *Event) error) WebhookOption {
	return func(w *WebhookTransport) { w.syncDispatchFunc = fn }
}

// WithWebhookErrorHandler receives asynchronous dispatch failures.
func WithWebhookErrorHandler(h ErrorHandler) WebhookOption {
	return func(w *WebhookTransport) { w.onError = h }
}

// WithWebhookInsecureSkipVerify disables signature verification.
//
// Only use it when something else already authenticates the callback, such as
// a trusted reverse proxy: an unverified webhook endpoint accepts forged
// events from anyone who can reach it.
func WithWebhookInsecureSkipVerify(skip bool) WebhookOption {
	return func(w *WebhookTransport) { w.allowUnverified = skip }
}

// NewWebhookTransport returns a webhook transport.
func NewWebhookTransport(opts ...WebhookOption) *WebhookTransport {
	w := &WebhookTransport{
		addr: DefaultWebhookAddr,
		path: DefaultWebhookPath,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Name implements Transport.
func (w *WebhookTransport) Name() string { return TransportNameWebhook }

// Path returns the callback path.
func (w *WebhookTransport) Path() string { return w.path }

// verifier builds the signature verifier, or reports why verification is
// impossible.
func (w *WebhookTransport) verifier() (*Signer, error) {
	if w.hasKey {
		return &Signer{publicKey: w.publicKey[:]}, nil
	}
	if w.secret == "" {
		return nil, errors.New("qqbotsdk: webhook needs a secret or a public key to verify callbacks")
	}
	return NewSigner(w.secret)
}

// Handler returns the http.Handler serving the callback path, so the caller can
// mount it on an existing server.
//
// Signature verification is configured the same way as for the standalone
// server; use WithWebhookInsecureSkipVerify to disable it.
func (w *WebhookTransport) Handler(dispatch DispatchFunc) (http.Handler, error) {
	if dispatch == nil {
		return nil, errors.New("qqbotsdk: webhook handler needs a dispatch function")
	}
	var signer *Signer
	if !w.allowUnverified {
		var err error
		if signer, err = w.verifier(); err != nil {
			return nil, err
		}
	}

	// When the caller asked for synchronous dispatch, fail loudly if the
	// dispatch function cannot provide it: silently acknowledging before the
	// handlers ran would defeat the purpose.
	var syncFn func(context.Context, *Event) error
	if w.syncDispatch {
		syncFn = w.synchronousDispatch()
		if syncFn == nil {
			return nil, errors.New("qqbotsdk: synchronous dispatch needs WithWebhookSyncFunc, or Client.Start")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc(w.path, func(rw http.ResponseWriter, r *http.Request) {
		w.serveCallback(rw, r, signer, dispatch, syncFn)
	})
	return mux, nil
}

// serveCallback handles one callback request.
func (w *WebhookTransport) serveCallback(rw http.ResponseWriter, r *http.Request, signer *Signer, dispatch DispatchFunc, syncFn func(context.Context, *Event) error) {
	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
	if err != nil {
		http.Error(rw, "cannot read body", http.StatusBadRequest)
		return
	}

	// Signature verification comes first: an unverified body must not be
	// parsed, let alone acted upon.
	if signer != nil {
		timestamp := r.Header.Get(SignatureTimestampHeader)
		if err := signer.Verify(timestamp, body, r.Header.Get(SignatureHeader)); err != nil {
			w.reportError(r.Context(), nil, fmt.Errorf("qqbotsdk: rejecting callback with bad signature: %w", err))
			http.Error(rw, "invalid signature", http.StatusUnauthorized)
			return
		}
	}
	if w.appID != "" {
		if got := r.Header.Get(BotAppIDHeader); got != "" && got != w.appID {
			w.reportError(r.Context(), nil, fmt.Errorf("qqbotsdk: rejecting callback for another app: %s", got))
			http.Error(rw, "wrong appid", http.StatusUnauthorized)
			return
		}
	}

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		return
	}

	switch payload.Op {
	case OpCallbackVerify:
		w.serveValidation(rw, &payload, signer)
		return
	case OpDispatch:
		// handled below
	default:
		// Unknown opcodes still get an acknowledgement, so the platform does
		// not keep retrying a message this SDK does not understand.
		w.writeACK(rw)
		return
	}

	event := NewEvent(&payload, TransportNameWebhook)

	if w.syncDispatch {
		if err := syncFn(r.Context(), event); err != nil {
			http.Error(rw, "handler failed", http.StatusInternalServerError)
			return
		}
		w.writeACK(rw)
		return
	}

	// Acknowledge first and handle afterwards: the platform retries a callback
	// it believes failed, and handler work must not delay the acknowledgement.
	dispatch(r.Context(), event)
	w.writeACK(rw)
}

// serveValidation answers an op 13 callback address validation request.
func (w *WebhookTransport) serveValidation(rw http.ResponseWriter, payload *Payload, signer *Signer) {
	if signer == nil {
		http.Error(rw, "validation requires signature support", http.StatusInternalServerError)
		return
	}

	var req ValidationRequest
	if err := payload.DecodeData(&req); err != nil {
		http.Error(rw, "invalid validation payload", http.StatusBadRequest)
		return
	}
	resp, err := signer.HandleValidation(&req)
	if err != nil {
		http.Error(rw, "cannot sign validation", http.StatusBadRequest)
		return
	}
	writeJSON(rw, http.StatusOK, resp)
}

// writeACK replies to a pushed event with the documented op 12 acknowledgement.
func (w *WebhookTransport) writeACK(rw http.ResponseWriter) {
	writeJSON(rw, http.StatusOK, &Payload{Op: OpHTTPCallbackACK})
}

// Start runs a standalone HTTP server receiving callbacks.
func (w *WebhookTransport) Start(ctx context.Context, dispatch DispatchFunc) error {
	if err := w.validateAddr(); err != nil {
		return err
	}
	handler, err := w.Handler(dispatch)
	if err != nil {
		return err
	}

	runCtx, err := w.begin(ctx)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", w.addr)
	if err != nil {
		w.finish()
		return fmt.Errorf("qqbotsdk: webhook listen on %s: %w", w.addr, err)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	w.mu.Lock()
	w.listener = listener
	w.server = server
	w.mu.Unlock()

	go func() {
		defer w.finish()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			w.reportError(context.Background(), nil, fmt.Errorf("qqbotsdk: webhook server stopped: %w", err))
		}
	}()

	// Shut the server down when the run context ends.
	go func() {
		<-runCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
		defer cancel()
		_ = w.stopServer(shutdownCtx)
	}()

	return nil
}

// validateAddr rejects a listen address on a port the platform never calls
// back on, which would otherwise look like a silently broken webhook.
//
// Port 0 is accepted: it asks the operating system for a free port, which suits
// tests and a process behind a load balancer that publishes one of the allowed
// ports. Read the chosen port back with Addr.
func (w *WebhookTransport) validateAddr() error {
	_, port, err := net.SplitHostPort(w.addr)
	if err != nil {
		return fmt.Errorf("qqbotsdk: webhook addr %q: %w", w.addr, err)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("qqbotsdk: webhook addr %q: invalid port: %w", w.addr, err)
	}
	if number == 0 {
		return nil
	}
	for _, allowed := range AllowedCallbackPorts {
		if number == allowed {
			return nil
		}
	}
	return fmt.Errorf("qqbotsdk: webhook port %d is not one of the allowed callback ports %v",
		number, AllowedCallbackPorts)
}

// Addr returns the address the server is listening on, which is useful when
// the configured address used port 0.
func (w *WebhookTransport) Addr() net.Addr {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.listener == nil {
		return nil
	}
	return w.listener.Addr()
}

// Stop shuts the server down.
func (w *WebhookTransport) Stop(ctx context.Context) error {
	if !w.isStarted() {
		return nil
	}
	err := w.stopServer(ctx)
	w.finish()
	return err
}

// stopServer shuts the HTTP server down.
func (w *WebhookTransport) stopServer(ctx context.Context) error {
	w.mu.Lock()
	server := w.server
	w.server = nil
	w.mu.Unlock()

	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

// reportError forwards a failure to the configured error handler, and to the
// logger when one is set.
//
// Transport-level failures have no other channel: a callback that failed
// signature verification cannot be reported through a handler.
func (w *WebhookTransport) reportError(ctx context.Context, event *Event, err error) {
	w.mu.Lock()
	onError := w.onError
	logger := w.logger
	w.mu.Unlock()

	if onError != nil {
		defer func() { _ = recover() }()
		onError(ctx, event, err)
	}
	if logger != nil {
		logger.Error("qqbotsdk: webhook transport error", "error", err)
	}
}

// installErrorHandler sets the error handler when the caller did not.
//
// Client.Start calls it before the transport starts; a caller wiring the
// transport by hand uses WithWebhookErrorHandler.
func (w *WebhookTransport) installErrorHandler(h ErrorHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.onError == nil {
		w.onError = h
	}
}

// synchronousDispatch returns the function that runs an event synchronously.
//
// Client.Start installs the dispatcher's own entry point; a caller wiring the
// transport by hand supplies one with WithWebhookSyncFunc.
func (w *WebhookTransport) synchronousDispatch() func(context.Context, *Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.syncDispatchFunc
}

// SetSyncDispatch installs the synchronous dispatch entry point. Client.Start
// calls it; a caller wiring the transport by hand may call it too.
func (w *WebhookTransport) SetSyncDispatch(fn func(context.Context, *Event) error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncDispatchFunc = fn
}

// writeJSON writes a JSON response, ignoring a write failure: the client is
// gone at that point and there is nothing useful left to do.
func writeJSON(rw http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(rw, "cannot encode response", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_, _ = rw.Write(data)
}
