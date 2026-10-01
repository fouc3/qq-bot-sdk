package qqbotsdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWithHTTPClientIsUsed(t *testing.T) {
	var used bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-H","expires_in":"7200"}`))
	}))
	t.Cleanup(srv.Close)

	hc := &http.Client{
		Timeout: 5 * time.Second,
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			used = true
			return http.DefaultTransport.RoundTrip(r)
		}),
	}
	client := NewClient("id", "secret", WithBaseURL(srv.URL), WithHTTPClient(hc))

	if _, err := client.GetAppAccessToken(context.Background()); err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}
	if !used {
		t.Error("the injected http.Client was not used")
	}
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestHTTPErrorBodyIsTruncated(t *testing.T) {
	long := strings.Repeat("x", 1000)
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(long))
	})

	_, err := client.GetAppAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if strings.Count(err.Error(), "x") > 300 {
		t.Errorf("error message embedded %d bytes of the body, want it truncated", strings.Count(err.Error(), "x"))
	}
	if !strings.Contains(err.Error(), "...") {
		t.Error("truncated body should be marked with an ellipsis")
	}
}

func TestGetAppAccessTokenNullExpiry(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-I","expires_in":null}`))
	})

	token, err := client.GetAppAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAppAccessToken: %v", err)
	}
	if token.ExpiresIn != 0 {
		t.Errorf("ExpiresIn = %v, want 0 for a null value", token.ExpiresIn)
	}
}

func TestGetAppAccessTokenInvalidExpiry(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-J","expires_in":"not-a-number"}`))
	})

	_, err := client.GetAppAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected a decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %v, want it to report a decode failure", err)
	}
}

func TestGetAppAccessTokenContextCancelled(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"TOKEN-K","expires_in":"7200"}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetAppAccessToken(ctx)
	if err == nil {
		t.Fatal("expected an error for a cancelled context, got nil")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("a transport failure must not decode as *APIError, got %v", apiErr)
	}
}
