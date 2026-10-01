package qqbotsdk

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// accessTokenPath is the endpoint that issues bot access tokens.
const accessTokenPath = "/app/getAppAccessToken"

// refreshMargin is how long before expiry a cached token is treated as stale.
//
// The platform starts handing out a new token once the current one is within
// 60 seconds of expiry, so a client that observes the same margin keeps its
// cached token valid and refreshes exactly when the platform allows it.
const refreshMargin = 60 * time.Second

// AccessToken is a bot access token together with its expiry.
type AccessToken struct {
	// Value is the token itself.
	Value string `json:"access_token"`
	// ExpiresIn is the lifetime reported by the platform, in seconds.
	ExpiresIn time.Duration `json:"-"`
	// ExpiresAt is the moment the token stops being valid.
	ExpiresAt time.Time `json:"-"`
}

// Expired reports whether the token has passed its expiry.
func (t *AccessToken) Expired() bool {
	return t.ExpiresAt.Before(time.Now())
}

// ValidN reports whether the token is still usable for n more seconds.
func (t *AccessToken) ValidN(d time.Duration) bool {
	return time.Now().Add(d).Before(t.ExpiresAt)
}

// accessTokenResponse mirrors the JSON body of the access token endpoint.
//
// expires_in is documented as a number but returned as a string in the
// official example, so it is decoded leniently.
type accessTokenResponse struct {
	AccessToken string          `json:"access_token"`
	ExpiresIn   flexibleSeconds `json:"expires_in"`
	Code        ErrorCode       `json:"code"`
	Message     string          `json:"message"`
}

// errCode returns the business error of the response, or nil when it succeeded.
func (r *accessTokenResponse) errCode() error {
	if r.Code == 0 {
		return nil
	}
	return &APIError{Code: r.Code, Message: r.Message}
}

// flexibleSeconds decodes a JSON value that may be either a number or a
// numeric string, and interprets it as seconds.
type flexibleSeconds time.Duration

func (s *flexibleSeconds) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = 0
		return nil
	}
	text := string(data)
	if len(text) > 1 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("qqbotsdk: invalid seconds value %s: %w", string(data), err)
	}
	*s = flexibleSeconds(time.Duration(n * float64(time.Second)))
	return nil
}

// GetAppAccessToken requests a new access token, bypassing any cache.
//
// Prefer AccessToken, which reuses a still-valid token. Use this only when a
// fresh token is explicitly required by the caller.
func (c *Client) GetAppAccessToken(ctx context.Context) (*AccessToken, error) {
	payload := struct {
		AppID        string `json:"appId"`
		ClientSecret string `json:"clientSecret"`
	}{
		AppID:        c.credentials.AppID,
		ClientSecret: c.credentials.ClientSecret,
	}

	var resp accessTokenResponse
	if err := c.postJSON(ctx, accessTokenPath, payload, &resp); err != nil {
		return nil, err
	}
	if err := resp.errCode(); err != nil {
		return nil, err
	}
	if resp.AccessToken == "" {
		return nil, fmt.Errorf("qqbotsdk: %s: empty access_token in response", accessTokenPath)
	}

	token := &AccessToken{
		Value:     resp.AccessToken,
		ExpiresIn: time.Duration(resp.ExpiresIn),
		ExpiresAt: time.Now().Add(time.Duration(resp.ExpiresIn)),
	}
	return token, nil
}

// AccessToken returns a usable access token, fetching one when the cached
// token is missing or about to expire.
//
// The token is refreshed refreshMargin before it lapses. Concurrent callers
// share a single in-flight request.
func (c *Client) AccessToken(ctx context.Context) (*AccessToken, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != nil && c.token.ValidN(refreshMargin) {
		return c.token, nil
	}

	token, err := c.GetAppAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	c.token = token
	return token, nil
}

// InvalidateToken drops the cached token so the next AccessToken call refetches.
func (c *Client) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = nil
}

// TokenSource hands out bot access tokens. It mirrors oauth2.TokenSource so a
// Client can be used where one is expected.
type TokenSource interface {
	Token(ctx context.Context) (*AccessToken, error)
}

// TokenSource returns a TokenSource backed by this client, satisfying the
// TokenSource interface without exposing the Client itself.
func (c *Client) TokenSource() TokenSource {
	return &clientTokenSource{client: c}
}

type clientTokenSource struct {
	client *Client
}

func (s *clientTokenSource) Token(ctx context.Context) (*AccessToken, error) {
	return s.client.AccessToken(ctx)
}

// AuthorizationHeader returns the value for the Authorization request header
// that authenticated OpenAPI calls must carry, e.g. "QQBot ACCESS_TOKEN".
func (t *AccessToken) AuthorizationHeader() string {
	return "QQBot " + t.Value
}
