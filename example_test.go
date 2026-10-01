package qqbotsdk_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// ExampleNewClientFromEnv shows the documented startup path: credentials come
// from the environment, and a missing pair fails immediately at startup rather
// than midway through a run.
func ExampleNewClientFromEnv() {
	// Reads ACCESS_TOKEN, or APPID plus CLIENTSECRET.
	client, err := qqbotsdk.NewClientFromEnv()
	if errors.Is(err, qqbotsdk.ErrNoCredentials) {
		log.Fatal("set ACCESS_TOKEN, or set both APPID and CLIENTSECRET")
	}
	if err != nil {
		log.Fatal(err)
	}
	_ = client
	fmt.Println("client ready")
}

// ExampleNewClient shows the explicit path, which enables automatic token
// refresh from the AppID and ClientSecret.
//
// It carries no Output comment on purpose: go test would otherwise run it and
// make a real network call.
func ExampleNewClient() {
	client := qqbotsdk.NewClient("APPID", "CLIENTSECRET")

	token, err := client.AccessToken(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// The header every OpenAPI call must carry.
	fmt.Println(token.AuthorizationHeader())
}

// ExampleWithHeader shows overriding a documented request header. The defaults
// are https://api.bot.qq.com, "QQBot <token>" and
// "application/json; charset=utf-8".
func ExampleWithHeader() {
	client := qqbotsdk.NewClient("APPID", "CLIENTSECRET",
		qqbotsdk.WithBaseURL("https://api.example.com"),
		qqbotsdk.WithHeader("Content-Type", "application/json"),
		qqbotsdk.WithHeaders(http.Header{"X-Custom": {"v"}}),
	)
	_ = client
	fmt.Println("overrides applied")
	// Output: overrides applied
}

// ExampleOpenAPIError shows how a failed call is reported, including the
// request address, the platform err_code and the trace id.
func ExampleOpenAPIError() {
	var err error // returned by a real API call

	var openAPIErr *qqbotsdk.OpenAPIError
	if errors.As(err, &openAPIErr) {
		switch {
		case openAPIErr.IsAuthFailure():
			// 401, or a token/appid error code: refresh and retry once.
		case openAPIErr.IsRateLimited():
			// 429 or a frequency-limit code: back off.
		case openAPIErr.Retryable():
			// A system error the documentation says may be retried once.
		}
		// Hand this to platform support when the cause is unclear.
		fmt.Println(openAPIErr.TraceID)
	}
}
