package qqbotsdk

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestRespondInteractionMatchesDocumentedRequest reproduces the documented call.
func TestRespondInteractionMatchesDocumentedRequest(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	err := client.RespondInteraction(t.Context(),
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890", InteractionCodeSuccess)
	if err != nil {
		t.Fatalf("RespondInteraction: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", req.Method)
	}
	want := "/interactions/a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	if got := req.EscapedPath; got != want {
		t.Errorf("path = %s\nwant       %s", got, want)
	}

	body := decodeBody(t, req)
	if len(body) != 1 || body["code"] != float64(InteractionCodeSuccess) {
		t.Errorf("body = %v, want only the code", body)
	}
}

// TestRespondInteractionReportsEveryCode checks that each documented result is
// sent as written.
func TestRespondInteractionReportsEveryCode(t *testing.T) {
	codes := map[string]InteractionCode{
		"success":       InteractionCodeSuccess,
		"failed":        InteractionCodeFailed,
		"too frequent":  InteractionCodeTooFrequent,
		"duplicate":     InteractionCodeDuplicate,
		"no permission": InteractionCodeNoPermission,
		"admin only":    InteractionCodeAdminOnly,
	}

	client, captured := newMessageServer(t, http.StatusOK, `{}`)
	for name, code := range codes {
		t.Run(name, func(t *testing.T) {
			if err := client.RespondInteraction(t.Context(), "INTER1", code); err != nil {
				t.Fatalf("RespondInteraction(%d): %v", code, err)
			}
			if got := decodeBody(t, last(t, captured))["code"]; got != float64(code) {
				t.Errorf("code = %v, want %d", got, code)
			}
		})
	}
}

// TestRespondInteractionStripsTheEventPrefix covers the prefix the
// documentation warns about: the endpoint takes the bare id.
func TestRespondInteractionStripsTheEventPrefix(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	cases := map[string]string{
		"bare":           "abc",
		"prefixed":       "INTERACTION_CREATE:abc",
		"space around":   "  abc  ",
		"prefixed+space": " INTERACTION_CREATE:abc ",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if err := client.RespondInteraction(t.Context(), input, InteractionCodeSuccess); err != nil {
				t.Fatalf("RespondInteraction: %v", err)
			}
			if got := last(t, captured).EscapedPath; got != "/interactions/abc" {
				t.Errorf("path = %s, want the bare id", got)
			}
		})
	}

	// The prefix is only stripped once, and only at the start.
	if err := client.RespondInteraction(t.Context(), "x-INTERACTION_CREATE:y", InteractionCodeSuccess); err != nil {
		t.Fatalf("RespondInteraction: %v", err)
	}
	if got := last(t, captured).EscapedPath; got != "/interactions/x-INTERACTION_CREATE:y" {
		t.Errorf("path = %s, want the inner occurrence left alone", got)
	}
}

// TestRespondInteractionEscapesTheID checks that an id cannot break the route.
func TestRespondInteractionEscapesTheID(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.RespondInteraction(t.Context(), "a/b?c=d", InteractionCodeSuccess); err != nil {
		t.Fatalf("RespondInteraction: %v", err)
	}
	req := last(t, captured)
	if !strings.Contains(req.EscapedPath, "%2F") {
		t.Errorf("path = %s, want the slash escaped", req.EscapedPath)
	}
	if req.Query != "" {
		t.Errorf("query = %q, want the question mark escaped rather than starting a query", req.Query)
	}
	if decoded, err := url.PathUnescape(req.EscapedPath); err != nil {
		t.Errorf("the escaped path is not reversible: %v", err)
	} else if decoded != "/interactions/a/b?c=d" {
		t.Errorf("decoded path = %s", decoded)
	}
}

// TestRespondInteractionValidatesInput checks the local guards, which keep a
// mistake from costing a round trip.
func TestRespondInteractionValidatesInput(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	invalid := map[string]struct {
		id   string
		code InteractionCode
	}{
		"no id":          {"", InteractionCodeSuccess},
		"blank id":       {"   ", InteractionCodeSuccess},
		"prefix only":    {"INTERACTION_CREATE:", InteractionCodeSuccess},
		"code below":     {"INTER1", InteractionCode(-1)},
		"code above":     {"INTER1", InteractionCode(6)},
		"code far above": {"INTER1", InteractionCode(500)},
	}
	for name, tc := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := client.RespondInteraction(t.Context(), tc.id, tc.code); err == nil {
				t.Error("expected a validation error")
			}
		})
	}

	if len(*captured) != 0 {
		t.Errorf("a rejected call still sent %d requests", len(*captured))
	}

	// The bounds themselves must be accepted.
	for _, code := range []InteractionCode{InteractionCodeSuccess, InteractionCodeAdminOnly} {
		if err := code.Validate(); err != nil {
			t.Errorf("Validate(%d) = %v, want the documented bounds to pass", code, err)
		}
	}
}

// TestInteractionCodeString pins the documented meanings.
func TestInteractionCodeString(t *testing.T) {
	cases := map[InteractionCode]string{
		InteractionCodeSuccess:      "success",
		InteractionCodeFailed:       "failed",
		InteractionCodeTooFrequent:  "too frequent",
		InteractionCodeDuplicate:    "duplicate",
		InteractionCodeNoPermission: "no permission",
		InteractionCodeAdminOnly:    "admin only",
		InteractionCode(9):          "InteractionCode(9)",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("InteractionCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestRespondInteractionPropagatesPlatformErrors checks that the conditions
// only the platform knows about reach the caller.
func TestRespondInteractionPropagatesPlatformErrors(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrInteractionParamInvalid:  "630001",
		ErrInteractionAppIDMismatch: "630003",
		ErrInteractionGetDataFailed: "630005",
	}
	for code, label := range cases {
		t.Run(label, func(t *testing.T) {
			client, _ := newMessageServer(t, http.StatusOK,
				`{"err_code":`+label+`,"message":"failed"}`)

			err := client.RespondInteraction(t.Context(), "INTER1", InteractionCodeSuccess)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !IsOpenAPIError(err, code) {
				t.Errorf("err = %v, want code %d", err, code)
			}
		})
	}
}

// TestInteractionErrorCodeNames pins the documented descriptions.
func TestInteractionErrorCodeNames(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrInteractionParamInvalid:      "param invalid",
		ErrInteractionAppIDFailed:       "get appid failed",
		ErrInteractionAppIDMismatch:     "appid invalid",
		ErrInteractionSetDataFailed:     "set interaction data failed",
		ErrInteractionGetDataFailed:     "get interaction data failed",
		ErrInteractionHeaderAppIDFailed: "get header appid failed",
		ErrInteractionDataTooLarge:      "data too large",
		ErrInteractionPreprocessFailed:  "interaction preprocess failed",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpenAPIErrorCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestRespondInteractionNeedsAMessageButton documents the pairing between the
// event helper and the response endpoint.
func TestRespondInteractionNeedsAMessageButton(t *testing.T) {
	button := &InteractionCreateData{Type: InteractionInlineKeyboard}
	if !button.NeedsResponse() {
		t.Error("a message button must be answered")
	}

	feedback := &InteractionCreateData{Type: InteractionMessageFeedback}
	if feedback.NeedsResponse() {
		t.Error("a message feedback must not be answered")
	}
}
