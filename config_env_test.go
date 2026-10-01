package qqbotsdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLoadConfigFromProcessEnv covers the os.LookupEnv entry point, which
// loadConfigFrom cannot exercise.
func TestLoadConfigFromProcessEnv(t *testing.T) {
	t.Setenv(EnvAppID, "env-appid")
	t.Setenv(EnvClientSecret, "env-secret")
	t.Setenv(EnvAccessToken, "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.AppID != "env-appid" || cfg.ClientSecret != "env-secret" {
		t.Errorf("cfg = %+v, want the environment values", cfg)
	}
}

func TestLoadConfigFromProcessEnvWithoutCredentials(t *testing.T) {
	t.Setenv(EnvAppID, "")
	t.Setenv(EnvClientSecret, "")
	t.Setenv(EnvAccessToken, "")

	if _, err := LoadConfig(); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

func TestNewClientFromEnv(t *testing.T) {
	t.Setenv(EnvAppID, "")
	t.Setenv(EnvClientSecret, "")
	t.Setenv(EnvAccessToken, "env-token")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv: %v", err)
	}
	token, err := client.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token.Value != "env-token" {
		t.Errorf("token = %q, want env-token", token.Value)
	}
}

func TestNewClientFromEnvWithoutCredentials(t *testing.T) {
	t.Setenv(EnvAppID, "")
	t.Setenv(EnvClientSecret, "")
	t.Setenv(EnvAccessToken, "")

	if _, err := NewClientFromEnv(); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

// TestNewClientFromEnvOptionOverridesEnv checks that options win over the
// environment, which is the documented layering.
func TestNewClientFromEnvOptionOverridesEnv(t *testing.T) {
	t.Setenv(EnvAppID, "")
	t.Setenv(EnvClientSecret, "")
	t.Setenv(EnvAccessToken, "env-token")

	client, err := NewClientFromEnv(WithHeader("Authorization", "Bearer CUSTOM"))
	if err != nil {
		t.Fatalf("NewClientFromEnv: %v", err)
	}

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client.baseURL = srv.URL

	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotAuth != "Bearer CUSTOM" {
		t.Errorf("Authorization = %q, want the option override", gotAuth)
	}
}

func TestWithAccessTokenOption(t *testing.T) {
	var tokenEndpointHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == accessTokenPath {
			tokenEndpointHit = true
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("id", "secret", WithBaseURL(srv.URL), WithAccessToken("OPTION-TOKEN"))
	token, err := client.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token.Value != "OPTION-TOKEN" {
		t.Errorf("token = %q, want OPTION-TOKEN", token.Value)
	}
	if tokenEndpointHit {
		t.Error("WithAccessToken must suppress the token request")
	}
}

func TestNewClientFromConfigAppliesBaseURLAndHeaders(t *testing.T) {
	var gotPath, gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCustom = r.Header.Get("X-Custom")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{
		AccessToken: "T",
		BaseURL:     srv.URL + "/", // trailing slash must be trimmed
		Headers:     http.Header{"X-Custom": {"v"}},
	})
	if err := client.doJSON(context.Background(), http.MethodGet, "/users/@me", nil, nil, openAPICall); err != nil {
		t.Fatalf("doJSON: %v", err)
	}
	if gotPath != "/users/@me" {
		t.Errorf("path = %q, want /users/@me", gotPath)
	}
	if gotCustom != "v" {
		t.Errorf("X-Custom = %q, want v", gotCustom)
	}
}

func TestRedact(t *testing.T) {
	if got := redact(""); got != "<unset>" {
		t.Errorf("redact(\"\") = %q, want <unset>", got)
	}
	if got := redact("s3cret"); got != "<set>" {
		t.Errorf("redact(secret) = %q, want <set>", got)
	}
}
