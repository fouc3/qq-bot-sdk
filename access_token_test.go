package qqbotsdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient points a Client at a test server.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("123456", "secret", WithBaseURL(srv.URL)), srv
}

func TestGetAppAccessTokenDecodesStringExpiry(t *testing.T) {
	var gotPath, gotContentType string
	var gotBody map[string]string

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-A","expires_in":"7200"}`))
	})

	token, err := client.GetAppAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}

	if gotPath != "/app/getAppAccessToken" {
		t.Errorf("path = %q, want %q", gotPath, "/app/getAppAccessToken")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["appId"] != "123456" {
		t.Errorf("appId = %q, want 123456", gotBody["appId"])
	}
	if gotBody["clientSecret"] != "secret" {
		t.Errorf("clientSecret = %q, want secret", gotBody["clientSecret"])
	}
	if token.Value != "TOKEN-A" {
		t.Errorf("Value = %q, want TOKEN-A", token.Value)
	}
	if token.ExpiresIn != 7200*time.Second {
		t.Errorf("ExpiresIn = %v, want 2h", token.ExpiresIn)
	}
}

func TestGetAppAccessTokenDecodesNumericExpiry(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-B","expires_in":7200}`))
	})

	token, err := client.GetAppAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}
	if token.ExpiresIn != 7200*time.Second {
		t.Errorf("ExpiresIn = %v, want 2h", token.ExpiresIn)
	}
}

func TestGetAppAccessTokenBusinessError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Business failures still use HTTP 200.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":100016,"message":"invalid appid or secret"}`))
	})

	_, err := client.GetAppAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %v is not an *APIError", err)
	}
	if apiErr.Code != ErrCodeInvalidCredential {
		t.Errorf("Code = %d, want %d", apiErr.Code, ErrCodeInvalidCredential)
	}
	if !strings.Contains(apiErr.Error(), "invalid appid or secret") {
		t.Errorf("Error() = %q, want it to carry the platform message", apiErr.Error())
	}
}

func TestGetAppAccessTokenHTTPError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	})

	_, err := client.GetAppAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("HTTP failures must not decode as *APIError, got %v", apiErr)
	}
}

func TestGetAppAccessTokenEmptyToken(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"","expires_in":"7200"}`))
	})

	if _, err := client.GetAppAccessToken(context.Background()); err == nil {
		t.Fatal("expected an error for an empty access_token, got nil")
	}
}

func TestAccessTokenCachesUntilNearExpiry(t *testing.T) {
	var calls int32
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-C","expires_in":"7200"}`))
	})

	ctx := context.Background()
	first, err := client.AccessToken(ctx)
	if err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}
	second, err := client.AccessToken(ctx)
	if err != nil {
		t.Fatalf("second AccessToken: %v", err)
	}

	if first != second {
		t.Error("a fresh token should be reused, got a different token")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("endpoint was called %d times, want 1", n)
	}
}

func TestAccessTokenRefreshesInsideMargin(t *testing.T) {
	var calls int32
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		// 30s left: inside the 60s refresh margin, so it must be refetched.
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-D","expires_in":"30"}`))
	})

	ctx := context.Background()
	if _, err := client.AccessToken(ctx); err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}
	if _, err := client.AccessToken(ctx); err != nil {
		t.Fatalf("second AccessToken: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("endpoint was called %d times, want 2 (token inside refresh margin)", n)
	}
}

func TestInvalidateTokenForcesRefetch(t *testing.T) {
	var calls int32
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-E","expires_in":"7200"}`))
	})

	ctx := context.Background()
	if _, err := client.AccessToken(ctx); err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}
	client.InvalidateToken()
	if _, err := client.AccessToken(ctx); err != nil {
		t.Fatalf("AccessToken after InvalidateToken: %v", err)
	}

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("endpoint was called %d times, want 2", n)
	}
}

func TestAccessTokenConcurrentCallersShareOneRequest(t *testing.T) {
	var calls int32
	release := make(chan struct{})
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		<-release // hold the request so all callers pile up behind the lock
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-F","expires_in":"7200"}`))
	})

	const goroutines = 8
	done := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := client.AccessToken(context.Background())
			done <- err
		}()
	}

	// Give the goroutines time to reach the lock, then let the request finish.
	time.Sleep(50 * time.Millisecond)
	close(release)

	for i := 0; i < goroutines; i++ {
		if err := <-done; err != nil {
			t.Fatalf("AccessToken: %v", err)
		}
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("endpoint was called %d times, want 1", n)
	}
}

func TestTokenAuthorizationHeader(t *testing.T) {
	token := &AccessToken{Value: "ACCESS_TOKEN"}
	if got, want := token.AuthorizationHeader(), "QQBot ACCESS_TOKEN"; got != want {
		t.Errorf("AuthorizationHeader() = %q, want %q", got, want)
	}
}

func TestTokenExpiryHelpers(t *testing.T) {
	fresh := &AccessToken{ExpiresAt: time.Now().Add(2 * time.Hour)}
	if fresh.Expired() {
		t.Error("a 2h token must not report Expired")
	}
	if !fresh.ValidN(time.Minute) {
		t.Error("a 2h token must be ValidN(1m)")
	}

	stale := &AccessToken{ExpiresAt: time.Now().Add(-time.Second)}
	if !stale.Expired() {
		t.Error("a past token must report Expired")
	}
}

func TestTokenSource(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-G","expires_in":"7200"}`))
	})

	var src TokenSource = client.TokenSource()
	token, err := src.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token.Value != "TOKEN-G" {
		t.Errorf("Value = %q, want TOKEN-G", token.Value)
	}
}
