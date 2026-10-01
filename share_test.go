package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// TestGenerateShareLinkMatchesDocumentedRequest reproduces the documented call.
func TestGenerateShareLinkMatchesDocumentedRequest(t *testing.T) {
	const wantURL = "https://qun.qq.com/qunpro/robot/qunshare?robot_appid=1234567890&robot_uin=12345678&data=xxx"
	client, captured := newMessageServer(t, http.StatusOK,
		`{"data":{"url":"`+wantURL+`"}}`)

	got, err := client.GenerateShareLink(t.Context(), "custom_data_123")
	if err != nil {
		t.Fatalf("GenerateShareLink: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/v2/generate_url_link" {
		t.Errorf("path = %s, want /v2/generate_url_link", req.Path)
	}
	if body := decodeBody(t, req); body["callback_data"] != "custom_data_123" {
		t.Errorf("callback_data = %v", body["callback_data"])
	}

	if got != wantURL {
		t.Errorf("url = %q\nwant     %q", got, wantURL)
	}
}

// TestGenerateShareLinkOmitsEmptyCallbackData checks that an empty value is not
// sent, since the field is optional.
func TestGenerateShareLinkOmitsEmptyCallbackData(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"data":{"url":"https://example.com/x"}}`)

	if _, err := client.GenerateShareLink(t.Context(), ""); err != nil {
		t.Fatalf("GenerateShareLink: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	if _, present := body["callback_data"]; present {
		t.Errorf("callback_data = %v, want it omitted", body["callback_data"])
	}
}

// TestGenerateShareLinkEnforcesCallbackLimit covers the documented 32 character
// limit, counted in characters rather than bytes so a multi-byte value is not
// rejected early.
func TestGenerateShareLinkEnforcesCallbackLimit(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"data":{"url":"https://example.com/x"}}`)

	// Exactly 32 characters, using multi-byte runes, must be accepted.
	atLimit := strings.Repeat("数", 32)
	if _, err := client.GenerateShareLink(t.Context(), atLimit); err != nil {
		t.Fatalf("a 32 character value must be accepted: %v", err)
	}

	// 33 characters must be rejected before any request is sent.
	before := len(*captured)
	if _, err := client.GenerateShareLink(t.Context(), strings.Repeat("数", 33)); err == nil {
		t.Error("a value over the documented limit must be rejected")
	}
	if len(*captured) != before {
		t.Error("an over-long value must be rejected without a round trip")
	}
}

// TestGenerateShareLinkRejectsEmptyResponseURL checks that a response without a
// link is reported rather than returned as an empty string.
func TestGenerateShareLinkRejectsEmptyResponseURL(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"data":{}}`)

	if _, err := client.GenerateShareLink(t.Context(), ""); err == nil {
		t.Error("an empty url must be reported")
	}
}

// TestGenerateShareLinkPropagatesError checks that a platform failure surfaces.
func TestGenerateShareLinkPropagates(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":11004,"message":"生成分享ARK失败"}`)

	_, err := client.GenerateShareLink(t.Context(), "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !IsOpenAPIError(err, ErrGenerateShareARKFailed) {
		t.Errorf("err = %v, want the share ark failure code", err)
	}
}

// TestShareErrorCodeNames pins the share link names, and documents that the
// common table keeps its own meaning for the numbers this endpoint reuses.
func TestShareErrorCodeNames(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrRequestHeaderInvalid:   "请求头异常",
		ErrUinFromHeaderFailed:    "从协议头获取uin失败",
		ErrGenerateShareARKFailed: "生成分享ARK失败",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpenAPIErrorCode(%d).String() = %q, want %q", code, got, want)
		}
	}

	// The common table still owns 10001 and 10003, which this endpoint's own
	// table reuses with different meanings.
	if got := OpenAPIErrorCode(10001).String(); got != "UnknownAccount" {
		t.Errorf("10001 = %q, want the common table name to win", got)
	}
	if got := OpenAPIErrorCode(10003).String(); got != "UnknownChannel" {
		t.Errorf("10003 = %q, want the common table name to win", got)
	}
}
