package qqbotsdk

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Environment variables read at startup.
//
// A client needs credentials from exactly one of two sources: the ACCESS_TOKEN
// variable, or the AppID/ClientSecret pair.
const (
	// EnvAppID holds the bot AppID, together with EnvClientSecret.
	EnvAppID = "APPID"
	// EnvClientSecret holds the bot ClientSecret, together with EnvAppID.
	EnvClientSecret = "CLIENTSECRET"
	// EnvAccessToken holds an already issued access token.
	EnvAccessToken = "ACCESS_TOKEN"
)

// ErrNoCredentials reports that no usable credential was configured.
//
// It is returned at startup rather than at the first API call, so a
// misconfigured process fails immediately instead of midway through a run.
var ErrNoCredentials = errors.New(
	"qqbotsdk: no credentials configured: set ACCESS_TOKEN, or set both APPID and CLIENTSECRET",
)

// Config holds the settings a Client is built from.
type Config struct {
	// AppID and ClientSecret enable the access token flow, including
	// automatic refresh.
	AppID        string
	ClientSecret string
	// AccessToken is an already issued token. When it is set, no token is
	// fetched and no refresh happens.
	AccessToken string
	// BaseURL overrides the unified request address. Empty means
	// DefaultBaseURL.
	BaseURL string
	// Headers overrides request headers. Nil means the documented defaults.
	Headers http.Header
}

// Validate reports whether the configuration can produce an authenticated
// client.
func (c Config) Validate() error {
	if strings.TrimSpace(c.AccessToken) != "" {
		return nil
	}
	if strings.TrimSpace(c.AppID) != "" && strings.TrimSpace(c.ClientSecret) != "" {
		return nil
	}
	return ErrNoCredentials
}

// LoadConfig reads credentials from the process environment.
//
// ACCESS_TOKEN takes precedence when it is set, so an operator can pin a token
// without unsetting the AppID and ClientSecret. Otherwise the AppID and
// ClientSecret pair is used, which enables automatic token refresh.
//
// The returned error wraps ErrNoCredentials when neither source is complete,
// so callers can test for it with errors.Is.
func LoadConfig() (Config, error) {
	return loadConfigFrom(os.LookupEnv)
}

// loadConfigFrom implements LoadConfig against an injectable lookup function,
// so the rules can be tested without touching the process environment.
func loadConfigFrom(lookup func(string) (string, bool)) (Config, error) {
	get := func(key string) string {
		value, _ := lookup(key)
		return strings.TrimSpace(value)
	}

	cfg := Config{
		AppID:        get(EnvAppID),
		ClientSecret: get(EnvClientSecret),
		AccessToken:  get(EnvAccessToken),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// NewClientFromEnv builds a Client from the process environment, so a program
// can start with nothing but APPID/CLIENTSECRET or ACCESS_TOKEN set.
//
// Options are applied on top of the environment, letting a caller override the
// request address or individual headers.
func NewClientFromEnv(opts ...Option) (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return NewClientFromConfig(cfg, opts...)
}

// NewClientFromConfig builds a Client from an explicit Config.
func NewClientFromConfig(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	c := newClient()
	if cfg.BaseURL != "" {
		c.baseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	if len(cfg.Headers) > 0 {
		WithHeaders(cfg.Headers)(c)
	}

	if cfg.AccessToken != "" {
		c.staticToken = cfg.AccessToken
	} else {
		c.credentials = Credentials{AppID: cfg.AppID, ClientSecret: cfg.ClientSecret}
	}

	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// String renders the configuration without revealing the secret or the token,
// so it is safe to write to a log line.
func (c Config) String() string {
	return fmt.Sprintf("qqbotsdk.Config{appID=%q, clientSecret=%s, accessToken=%s, baseURL=%q}",
		c.AppID, redact(c.ClientSecret), redact(c.AccessToken), c.BaseURL)
}

// redact describes a secret without disclosing it.
func redact(secret string) string {
	if secret == "" {
		return "<unset>"
	}
	return "<set>"
}
