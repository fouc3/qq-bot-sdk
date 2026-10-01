package qqbotsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Defaults taken from the OpenAPI call guide. Every one of them can be
// overridden through an Option.
const (
	// DefaultBaseURL is the unified request address of the OpenAPI.
	DefaultBaseURL = "https://api.bot.qq.com"
	// DefaultAuthorizationScheme is the scheme prefix of the Authorization
	// header, documented as "QQBot {ACCESS_TOKEN}".
	DefaultAuthorizationScheme = "QQBot"
	// DefaultContentType is the documented Content-Type of OpenAPI requests.
	DefaultContentType = "application/json; charset=utf-8"
)

// TraceIDHeader carries the platform trace id on OpenAPI responses. The same
// value is also returned as the trace_id body field.
const TraceIDHeader = "X-Tps-trace-ID"

// defaultTimeout bounds a single HTTP request when no client is supplied.
const defaultTimeout = 15 * time.Second

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 1 << 20

// Credentials identifies a bot on the QQ Open Platform.
type Credentials struct {
	// AppID is the bot ID, issued when the bot is created on the platform.
	AppID string
	// ClientSecret is the bot secret. The console also calls it AppSecret.
	ClientSecret string
}

// Client talks to the QQ Bot OpenAPI and receives gateway events.
//
// A Client is safe for concurrent use.
type Client struct {
	credentials Credentials
	// staticToken is set when the caller supplies an access token directly
	// instead of letting the client fetch and refresh one.
	staticToken string
	baseURL     string
	httpClient  *http.Client
	// headers holds caller overrides. It is merged over the documented
	// defaults when a request is built.
	headers http.Header

	mu    sync.Mutex
	token *AccessToken

	// dispatcher routes received events to registered handlers. It is never
	// nil, so Register works on a Client built any way.
	dispatcher *Dispatcher
	// transports are started by Start, in order.
	transports []Transport
	// running reports whether Start has been called and Stop has not.
	running bool
	// runCancel cancels the context the transports run under.
	runCancel context.CancelFunc
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL replaces the API base URL. It is mainly useful in tests and for
// pointing at a proxy or a mock server.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient replaces the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithAccessToken makes the client authenticate with a fixed access token
// instead of fetching and refreshing one from the AppID and ClientSecret.
func WithAccessToken(token string) Option {
	return func(c *Client) { c.staticToken = token }
}

// WithHeader overrides a single request header. Header names are matched
// case-insensitively, so WithHeader("content-type", ...) overrides the
// documented default as well.
//
// Overriding Authorization disables the built-in credential handling: the
// given value is sent as-is and no access token is fetched.
func WithHeader(key, value string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = make(http.Header)
		}
		c.headers.Set(key, value)
	}
}

// WithHeaders overrides several request headers at once.
func WithHeaders(headers http.Header) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = make(http.Header, len(headers))
		}
		for key, values := range headers {
			c.headers[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
		}
	}
}

// WithDispatcher replaces the event dispatcher, for example to install an
// error handler or a concurrency limit.
func WithDispatcher(d *Dispatcher) Option {
	return func(c *Client) {
		if d != nil {
			c.dispatcher = d
		}
	}
}

// NewClient returns a Client bound to the given bot credentials.
func NewClient(appID, clientSecret string, opts ...Option) *Client {
	c := newClient()
	c.credentials = Credentials{AppID: appID, ClientSecret: clientSecret}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// newClient returns a Client carrying only the documented defaults.
func newClient() *Client {
	return &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		headers:    make(http.Header),
		dispatcher: NewDispatcher(),
	}
}

// callKind selects the error-reporting contract of an endpoint.
type callKind int

const (
	// credentialCall is POST /app/getAppAccessToken: failures arrive as the
	// "code" field of an HTTP 200 body.
	credentialCall callKind = iota
	// openAPICall is a regular OpenAPI endpoint: failures arrive as the
	// "err_code" field, and the HTTP status code is meaningful as well.
	openAPICall
)

// requestHeaders builds the header set of one request: the documented
// defaults, then the caller's overrides, then an Authorization header derived
// from the configured credential when the caller did not override it.
//
// authenticated is false for endpoints that are called before a credential
// exists, such as the access token endpoint itself.
func (c *Client) requestHeaders(ctx context.Context, authenticated bool) (http.Header, error) {
	headers := make(http.Header, len(c.headers)+2)
	for key, values := range c.headers {
		headers[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
	}
	if headers.Get("Content-Type") == "" {
		headers.Set("Content-Type", DefaultContentType)
	}
	if !authenticated || headers.Get("Authorization") != "" {
		return headers, nil
	}

	authorization, err := c.authorization(ctx)
	if err != nil {
		return nil, err
	}
	headers.Set("Authorization", authorization)
	return headers, nil
}

// authorization returns the Authorization header value of an OpenAPI call.
func (c *Client) authorization(ctx context.Context) (string, error) {
	if c.staticToken != "" {
		return DefaultAuthorizationScheme + " " + c.staticToken, nil
	}
	if c.credentials.AppID != "" && c.credentials.ClientSecret != "" {
		token, err := c.AccessToken(ctx)
		if err != nil {
			return "", err
		}
		return token.AuthorizationHeader(), nil
	}
	return "", ErrNoCredentials
}

// doJSON sends payload as a JSON body and decodes the JSON response into out.
//
// A nil payload sends no body, and a nil out discards the response body.
func (c *Client) doJSON(ctx context.Context, method, path string, payload, out any, kind callKind) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("qqbotsdk: encode request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("qqbotsdk: build request: %w", err)
	}

	headers, err := c.requestHeaders(ctx, kind == openAPICall)
	if err != nil {
		return err
	}
	req.Header = headers

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qqbotsdk: %s %s: %w", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("qqbotsdk: read response: %w", err)
	}

	if kind == openAPICall {
		if err := openAPIError(method, url, resp, data); err != nil {
			return err
		}
	} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("qqbotsdk: %s %s: unexpected status %s: %s", method, url, resp.Status, snippet(data))
	}

	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("qqbotsdk: decode response: %w", err)
	}
	return nil
}

// snippet renders at most maxSnippet bytes of a response body for error messages.
func snippet(data []byte) string {
	const maxSnippet = 256
	if len(data) > maxSnippet {
		return string(data[:maxSnippet]) + "..."
	}
	return string(data)
}
