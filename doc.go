// Package qqbotsdk is a Go SDK for the QQ Bot OpenAPI v2.
//
// Official documentation: https://bot.q.qq.com/wiki/develop/api-v2/
//
// # Layout
//
// The SDK is deliberately a single package: every public type, and every method
// it exposes, lives here. Go requires a method to be declared in the same
// package as its receiver, so splitting the API into subpackages would force
// several client types, or free functions taking a client, instead of one
// Client. The files are grouped by responsibility behind a prefix, so the
// directory reads in layers while still sorting together:
//
//	client.go, config.go, auth.go, errors.go, doc.go
//	    Client, credentials and token caching, configuration from the
//	    environment, and the error types.
//
//	api_*.go
//	    One file per endpoint domain, each holding the methods on Client:
//	    api_gateway, api_message, api_file, api_reaction, api_bot,
//	    api_share, api_menu, api_panel.
//
//	event.go, event_dispatcher.go, event_transport.go
//	    The gateway payload model and subscription intents, the handler
//	    dispatcher, and the Transport interface with the client lifecycle.
//
//	transport_webhook.go, transport_websocket.go, transport_sign.go
//	    The two event delivery implementations, and the Ed25519 signing used
//	    to verify callbacks.
//
//	errcode.go, errcode_message.go, errcode_share.go, errcode_menupanel.go
//	    The error code tables: the common one from the API call guide, then
//	    one per endpoint family.
//
// # Getting started
//
// Credentials come from the environment, or are passed explicitly:
//
//	client, err := qqbotsdk.NewClientFromEnv()
//	if errors.Is(err, qqbotsdk.ErrNoCredentials) {
//		log.Fatal("set ACCESS_TOKEN, or set both APPID and CLIENTSECRET")
//	}
//
// Calls read as methods on that one client:
//
//	token, err := client.AccessToken(ctx)
//	msg, err := client.SendC2CMessage(ctx, userOpenID, &qqbotsdk.Message{Content: "hi"})
//	menu, err := client.GetMenu(ctx)
//
// Events are delivered through transports and routed by a dispatcher:
//
//	client.UseTransport(qqbotsdk.NewWebSocketTransport(gatewayURL,
//		qqbotsdk.WithIntents(qqbotsdk.IntentPublicGuildMessages)))
//	client.Register(qqbotsdk.EventAtMessageCreate, handler)
//	client.Start(ctx)
//
// # Stability
//
// This package is under development; its public API is not yet stable.
package qqbotsdk
