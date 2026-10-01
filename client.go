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

// DefaultBaseURL is the production endpoint of the QQ Bot OpenAPI.
const DefaultBaseURL = "https://api.bot.qq.com"

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

// Client talks to the QQ Bot OpenAPI.
//
// A Client is safe for concurrent use.
type Client struct {
	credentials Credentials
	baseURL     string
	httpClient  *http.Client

	mu    sync.Mutex
	token *AccessToken
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL replaces the API base URL. It is mainly useful in tests.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient replaces the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient returns a Client bound to the given bot credentials.
func NewClient(appID, clientSecret string, opts ...Option) *Client {
	c := &Client{
		credentials: Credentials{AppID: appID, ClientSecret: clientSecret},
		baseURL:     DefaultBaseURL,
		httpClient:  &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// postJSON sends payload as a JSON body and decodes the JSON response into out.
//
// A non-2xx status is returned as a plain error. The OpenAPI reports business
// failures with HTTP 200 and a code in the body, so callers must inspect the
// decoded response as well.
func (c *Client) postJSON(ctx context.Context, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("qqbotsdk: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("qqbotsdk: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qqbotsdk: %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("qqbotsdk: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("qqbotsdk: %s: unexpected status %s: %s", path, resp.Status, snippet(data))
	}
	if out == nil {
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
