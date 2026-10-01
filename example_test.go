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

// ExampleClient_Register shows subscribing to events and starting the
// configured transports. Both delivery methods share the same Payload, so the
// handler does not care which one is active.
//
// It carries no Output comment on purpose: go test would otherwise run it and
// open a real connection.
func ExampleClient_Register() {
	client, err := qqbotsdk.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Gateway URL comes from GetGateway or GetGatewayBot.
	client.UseTransport(qqbotsdk.NewWebSocketTransport("wss://api.bot.qq.com/websocket/",
		qqbotsdk.WithIntents(qqbotsdk.IntentsFor(
			qqbotsdk.IntentPublicGuildMessages,
			qqbotsdk.IntentGroupAndC2CEvent,
		)),
	))

	registration := client.Register(qqbotsdk.EventGroupAtMessageCreate,
		qqbotsdk.EventHandlerFunc(func(ctx context.Context, event *qqbotsdk.Event) error {
			// The body shape depends on the event type.
			var data struct {
				Content string `json:"content"`
			}
			if err := event.DecodeData(&data); err != nil {
				return err
			}
			fmt.Println("received:", data.Content)
			return nil
		}),
	)
	defer registration.Cancel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Stop(context.Background()) }()
}

// ExampleSigner_Verify shows verifying a webhook callback. Verification must
// happen before the body is trusted or parsed.
func ExampleSigner_Verify() {
	signer, err := qqbotsdk.NewSigner("BOT_SECRET")
	if err != nil {
		log.Fatal(err)
	}

	var (
		timestamp    string // X-Signature-Timestamp
		body         []byte // raw request body
		signatureHex string // X-Signature-Ed25519
	)
	if err := signer.Verify(timestamp, body, signatureHex); errors.Is(err, qqbotsdk.ErrInvalidSignature) {
		fmt.Println("rejected")
		return
	}
	fmt.Println("verified")
}
