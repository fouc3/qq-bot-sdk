package qqbotsdk

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// lookupFrom builds an environment lookup from a map.
func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
}

func TestLoadConfigAcceptsAppIDAndSecret(t *testing.T) {
	cfg, err := loadConfigFrom(lookupFrom(map[string]string{
		EnvAppID:        "123456",
		EnvClientSecret: "secret",
	}))
	if err != nil {
		t.Fatalf("loadConfigFrom: %v", err)
	}
	if cfg.AppID != "123456" || cfg.ClientSecret != "secret" {
		t.Errorf("cfg = %+v, want the app credentials", cfg)
	}
	if cfg.AccessToken != "" {
		t.Errorf("AccessToken = %q, want empty", cfg.AccessToken)
	}
}

func TestLoadConfigAcceptsAccessTokenAlone(t *testing.T) {
	cfg, err := loadConfigFrom(lookupFrom(map[string]string{
		EnvAccessToken: "TOKEN",
	}))
	if err != nil {
		t.Fatalf("loadConfigFrom: %v", err)
	}
	if cfg.AccessToken != "TOKEN" {
		t.Errorf("AccessToken = %q, want TOKEN", cfg.AccessToken)
	}
}

func TestLoadConfigPrefersAccessToken(t *testing.T) {
	cfg, err := loadConfigFrom(lookupFrom(map[string]string{
		EnvAccessToken:  "TOKEN",
		EnvAppID:        "123456",
		EnvClientSecret: "secret",
	}))
	if err != nil {
		t.Fatalf("loadConfigFrom: %v", err)
	}
	if cfg.AccessToken != "TOKEN" {
		t.Errorf("AccessToken = %q, want the explicit token to win", cfg.AccessToken)
	}
}

func TestLoadConfigRejectsIncompleteCredentials(t *testing.T) {
	cases := map[string]map[string]string{
		"nothing set":       {},
		"appid only":        {EnvAppID: "123456"},
		"secret only":       {EnvClientSecret: "secret"},
		"blank values":      {EnvAppID: "   ", EnvClientSecret: "  "},
		"blank token":       {EnvAccessToken: " "},
		"token plus appid":  {EnvAccessToken: " ", EnvAppID: "123456"},
		"token plus secret": {EnvAccessToken: "", EnvClientSecret: "secret"},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigFrom(lookupFrom(env))
			if !errors.Is(err, ErrNoCredentials) {
				t.Fatalf("err = %v, want ErrNoCredentials", err)
			}
		})
	}
}

func TestLoadConfigTrimsWhitespace(t *testing.T) {
	cfg, err := loadConfigFrom(lookupFrom(map[string]string{
		EnvAppID:        "  123456  ",
		EnvClientSecret: "\tsecret\n",
	}))
	if err != nil {
		t.Fatalf("loadConfigFrom: %v", err)
	}
	if cfg.AppID != "123456" || cfg.ClientSecret != "secret" {
		t.Errorf("cfg = %+v, want trimmed values", cfg)
	}
}

func TestNewClientFromConfigWithStaticTokenSendsIt(t *testing.T) {
	var gotAuth string
	var tokenEndpointHit bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accessTokenPath {
			tokenEndpointHit = true
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClientFromConfig(Config{
		AccessToken: "STATIC",
		BaseURL:     srv.URL,
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}

	// The token endpoint must never be called when a static token is set.
	token, err := client.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token.Value != "STATIC" {
		t.Errorf("token = %q, want STATIC", token.Value)
	}
	if tokenEndpointHit {
		t.Error("a static token must not trigger a token request")
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty (AccessToken does not send requests)", gotAuth)
	}
}

func TestNewClientFromConfigRejectsNoCredentials(t *testing.T) {
	if _, err := NewClientFromConfig(Config{}); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

func TestConfigStringRedactsSecrets(t *testing.T) {
	cfg := Config{AppID: "123456", ClientSecret: "super-secret", AccessToken: "also-secret"}
	got := cfg.String()

	for _, secret := range []string{"super-secret", "also-secret"} {
		if strings.Contains(got, secret) {
			t.Errorf("String() leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "123456") {
		t.Errorf("String() should keep the AppID, got %s", got)
	}
}

func TestConfigValidate(t *testing.T) {
	valid := []Config{
		{AccessToken: "TOKEN"},
		{AppID: "1", ClientSecret: "s"},
		{AccessToken: "TOKEN", AppID: "1"},
	}
	for _, cfg := range valid {
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", cfg, err)
		}
	}

	invalid := []Config{
		{},
		{AppID: "1"},
		{ClientSecret: "s"},
		{AccessToken: "  "},
	}
	for _, cfg := range invalid {
		if err := cfg.Validate(); !errors.Is(err, ErrNoCredentials) {
			t.Errorf("Validate(%+v) = %v, want ErrNoCredentials", cfg, err)
		}
	}
}

func TestOpenAPICallUsesFetchedToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accessTokenPath {
			_, _ = w.Write([]byte(`{"access_token":"FETCHED","expires_in":"7200"}`))
			return
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("id", "secret", WithBaseURL(srv.URL))
	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotAuth != "QQBot FETCHED" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "QQBot FETCHED")
	}
}

func TestOpenAPICallWithoutCredentials(t *testing.T) {
	client := NewClient("", "", WithBaseURL("http://127.0.0.1:1"))
	err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall)
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

func TestHeaderOverrideChangesContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"access_token":"T","expires_in":"7200"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("id", "secret",
		WithBaseURL(srv.URL),
		WithHeader("content-type", "application/json"),
	)
	if _, err := client.GetAppAccessToken(context.Background()); err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want the override to win", gotContentType)
	}
}

func TestHeaderOverrideOfAuthorizationDisablesTokenFetch(t *testing.T) {
	var gotAuth string
	var tokenEndpointHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accessTokenPath {
			tokenEndpointHit = true
			_, _ = w.Write([]byte(`{"access_token":"FETCHED","expires_in":"7200"}`))
			return
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("id", "secret",
		WithBaseURL(srv.URL),
		WithHeader("Authorization", "Bearer CUSTOM"),
	)
	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotAuth != "Bearer CUSTOM" {
		t.Errorf("Authorization = %q, want the caller override", gotAuth)
	}
	if tokenEndpointHit {
		t.Error("an overridden Authorization must not trigger a token request")
	}
}

func TestWithHeadersAppliesSeveralOverrides(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"access_token":"T","expires_in":"7200"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("id", "secret",
		WithBaseURL(srv.URL),
		WithHeaders(http.Header{"X-Custom": {"one", "two"}, "Content-Type": {"application/json"}}),
	)
	if _, err := client.GetAppAccessToken(context.Background()); err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}
	if values := got.Values("X-Custom"); len(values) != 2 || values[0] != "one" {
		t.Errorf("X-Custom = %v, want [one two]", values)
	}
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want the override", got.Get("Content-Type"))
	}
}

// TestOpenAPIErrorRoundTrip drives a regular OpenAPI call end to end and checks
// the error the caller receives.
func TestOpenAPIErrorRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(TraceIDHeader, "header-trace")
		_, _ = w.Write([]byte(`{"err_code":40034005,"message":"回复消息msg_id已过期","trace_id":"body-trace"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})

	var out struct{}
	err := client.doJSON(context.Background(), http.MethodPost, "/v2/users/u/messages", map[string]any{"content": "hi"}, &out, openAPICall)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var openAPIErr *OpenAPIError
	if !errors.As(err, &openAPIErr) {
		t.Fatalf("err = %v, want *OpenAPIError", err)
	}
	if openAPIErr.Code != 40034005 {
		t.Errorf("Code = %d, want 40034005", openAPIErr.Code)
	}
	if openAPIErr.TraceID != "body-trace" {
		t.Errorf("TraceID = %q, want the body value to win", openAPIErr.TraceID)
	}
	if openAPIErr.URL != srv.URL+"/v2/users/u/messages" {
		t.Errorf("URL = %q, want the full request address", openAPIErr.URL)
	}
	if openAPIErr.Method != http.MethodPost {
		t.Errorf("Method = %q, want POST", openAPIErr.Method)
	}
	if !strings.Contains(err.Error(), "40034005") || !strings.Contains(err.Error(), "body-trace") {
		t.Errorf("Error() = %q, want code and trace id", err.Error())
	}
}

// NewClientFromConfigMust builds a client or fails the test.
func NewClientFromConfigMust(t *testing.T, cfg Config) *Client {
	t.Helper()
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}
	return client
}

// TestOpenAPISuccessWithErrorShapedBody ensures a zero err_code is not an error.
func TestOpenAPISuccessWithErrorShapedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"err_code":0,"message":"","trace_id":"t"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("err_code 0 must not be an error, got %v", err)
	}
}

// TestOpenAPICallSendsNoBodyForNilPayload checks the body-less request path.
func TestOpenAPICallSendsNoBodyForNilPayload(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty", body)
	}
}

// TestOpenAPICallEmptySuccessBody checks that a 204 with no body is fine.
func TestOpenAPICallEmptySuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	var out map[string]any
	if err := client.doJSON(context.Background(), http.MethodDelete, "/x", nil, &out, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if out != nil {
		t.Errorf("out = %v, want it left untouched", out)
	}
}
