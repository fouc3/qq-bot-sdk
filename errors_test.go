package qqbotsdk

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPIErrorCodeString(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrWrongToken:                 "ErrorWrongToken",
		ErrUnknownGuild:               "UnknownGuild",
		ErrSafeMessageRateLimited:     "安全打击：消息被限频",
		ErrPushMessageAsyncOK:         "PUSH_MSG_ASYNC_OK 推送消息异步调用成功，等待人工审核",
		OpenAPIErrorCode(0):           "",
		OpenAPIErrorCode(99999999999): "",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpenAPIErrorCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestEveryNamedCodeHasAnEntry guards against a constant being added without a
// name, which would silently render as a bare number.
func TestEveryNamedCodeHasAnEntry(t *testing.T) {
	codes := []OpenAPIErrorCode{
		ErrUnknownAccount, ErrUnknownChannel, ErrUnknownGuild,
		ErrCheckAdminFailed, ErrCheckAdminNotPass,
		ErrWrongAppID, ErrCheckAppPrivilegeFailed, ErrCheckAppPrivilegeNotPass,
		ErrInterfaceForbidden, ErrMissingAppID, ErrCheckRobot, ErrCheckGuildAuth,
		ErrGuildAuthNotPass, ErrRobotHasBanned,
		ErrWrongToken, ErrCheckTokenFailed, ErrCheckTokenNotPass,
		ErrCheckUserAuth, ErrUserAuthNotPass, ErrNoAppID,
		ErrGetHTTPHeader, ErrGetHeaderUIN, ErrGetNick, ErrGetAvatar,
		ErrGetGuildID, ErrGetGuildInfo,
		ErrReplaceIDFailed, ErrRequestInvalid, ErrResponseInvalid,
		ErrChannelHitWriteRateLimit,
		ErrCannotSendEmptyMessage, ErrInvalidFormBody, ErrMarkdownCombination,
		ErrNotSameChannel, ErrGetMessageFailed, ErrMessageTemplateTypeInvalid,
		ErrMarkdownEmpty, ErrMarkdownListTooLong, ErrGuildIDConvertFailed,
		ErrReplySelfMessage, ErrNotAtBotMessage, ErrNotBotMessage,
		ErrMessageIDEmpty, ErrOnlyKeyboardEditable, ErrKeyboardEmpty,
		ErrOnlyOwnMessageEditable, ErrModifyMessageFailed, ErrMarkdownTemplateParam,
		ErrInvalidMarkdownContent, ErrMarkdownNotAllowed, ErrMarkdownSyntaxConflict,
		ErrURLCallRetractParamInvalid, ErrMsgIDError, ErrGetMessageRetry,
		ErrNoPermissionDeleteMessage, ErrRetractMessageFailed, ErrGetChannelFailed,
		ErrSafeMessageRateLimited, ErrSafeMessageSensitive, ErrSafeNoExperience,
		ErrSafeHit, ErrSafeGroupGone, ErrInternalSystem, ErrCallerNotGroupMember,
		ErrGetChannelNameFailed, ErrHomeChannelNotAdmin, ErrAtAuthFailed,
		ErrTinyIDToUinFailed, ErrNotPrivateChannelMember, ErrNotWhitelistAppChannel,
		ErrTriggerChannelRateLimit, ErrOtherError, ErrEditMessageSafeHit,
		ErrPushMessageAsyncOK, ErrReplyMessageAsyncOK,
	}
	for _, code := range codes {
		if code.String() == "" {
			t.Errorf("code %d has no documented name", code)
		}
	}
}

func TestOpenAPIErrorHelpers(t *testing.T) {
	auth := &OpenAPIError{StatusCode: http.StatusUnauthorized}
	if !auth.IsAuthFailure() {
		t.Error("HTTP 401 must be an auth failure")
	}

	tokenErr := &OpenAPIError{Code: ErrWrongToken}
	if !tokenErr.IsAuthFailure() {
		t.Error("ErrorWrongToken must be an auth failure")
	}

	notFound := &OpenAPIError{StatusCode: http.StatusNotFound}
	if !notFound.IsNotFound() {
		t.Error("HTTP 404 must be IsNotFound")
	}
	if notFound.IsAuthFailure() {
		t.Error("HTTP 404 must not be an auth failure")
	}

	method := &OpenAPIError{StatusCode: http.StatusMethodNotAllowed}
	if !method.IsMethodNotAllowed() {
		t.Error("HTTP 405 must be IsMethodNotAllowed")
	}

	httpLimit := &OpenAPIError{StatusCode: http.StatusTooManyRequests}
	if !httpLimit.IsRateLimited() {
		t.Error("HTTP 429 must be rate limited")
	}
	codeLimit := &OpenAPIError{Code: ErrChannelHitWriteRateLimit}
	if !codeLimit.IsRateLimited() {
		t.Error("ChannelHitWriteRateLimit must be rate limited")
	}

	if !(&OpenAPIError{StatusCode: http.StatusInternalServerError}).IsServerError() {
		t.Error("HTTP 500 must be a server error")
	}
	if !(&OpenAPIError{StatusCode: http.StatusGatewayTimeout}).IsServerError() {
		t.Error("HTTP 504 must be a server error")
	}

	if !(&OpenAPIError{StatusCode: http.StatusCreated}).IsAsyncAccepted() {
		t.Error("HTTP 201 must be IsAsyncAccepted")
	}
	if !(&OpenAPIError{StatusCode: http.StatusAccepted}).IsAsyncAccepted() {
		t.Error("HTTP 202 must be IsAsyncAccepted")
	}
}

func TestOpenAPIErrorRetryable(t *testing.T) {
	retryable := []*OpenAPIError{
		{StatusCode: http.StatusTooManyRequests},
		{StatusCode: http.StatusInternalServerError},
		{StatusCode: http.StatusGatewayTimeout},
		{Code: ErrCheckAdminFailed},
		{Code: ErrCheckAppPrivilegeFailed},
		{Code: ErrCheckGuildAuth},
		{Code: ErrCheckTokenFailed},
		{Code: ErrGetMessageFailed},
		{Code: ErrRetractMessageFailed},
		{Code: ErrGetChannelFailed},
	}
	for _, e := range retryable {
		if !e.Retryable() {
			t.Errorf("%+v should be retryable", e)
		}
	}

	notRetryable := []*OpenAPIError{
		{StatusCode: http.StatusUnauthorized},
		{StatusCode: http.StatusNotFound},
		{Code: ErrCheckTokenNotPass},
		{Code: ErrSafeHit},
		{},
	}
	for _, e := range notRetryable {
		if e.Retryable() {
			t.Errorf("%+v should not be retryable", e)
		}
	}
}

func TestOpenAPIErrorIsStatus(t *testing.T) {
	e := &OpenAPIError{StatusCode: http.StatusNotFound}
	if !e.IsStatus(http.StatusNotFound) {
		t.Error("IsStatus should match the carried status")
	}
	if e.IsStatus(http.StatusUnauthorized) {
		t.Error("IsStatus should not match a different status")
	}
}

func TestOpenAPIErrorHasCodeAndIsOpenAPIError(t *testing.T) {
	err := error(&OpenAPIError{Code: ErrSafeHit})

	if !IsOpenAPIError(err, ErrSafeHit) {
		t.Error("IsOpenAPIError should match the code")
	}
	if IsOpenAPIError(err, ErrWrongToken) {
		t.Error("IsOpenAPIError should not match a different code")
	}
	if IsOpenAPIError(errors.New("plain"), ErrSafeHit) {
		t.Error("IsOpenAPIError must not match a plain error")
	}

	var openAPIErr *OpenAPIError
	if !errors.As(err, &openAPIErr) || !openAPIErr.HasCode(ErrSafeHit) {
		t.Error("HasCode should report the carried code")
	}
}

func TestOpenAPIErrorErrorText(t *testing.T) {
	full := &OpenAPIError{
		Method:     http.MethodPost,
		URL:        "https://api.bot.qq.com/v2/users/u/messages",
		StatusCode: http.StatusUnauthorized,
		Code:       ErrWrongToken,
		Message:    "token 错误",
		TraceID:    "trace-1",
	}
	got := full.Error()
	for _, want := range []string{"POST", "https://api.bot.qq.com/v2/users/u/messages", "401", "11241", "ErrorWrongToken", "token 错误", "trace-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, missing %q", got, want)
		}
	}

	sparse := &OpenAPIError{URL: "https://api.bot.qq.com/x"}
	if got := sparse.Error(); got == "" {
		t.Error("Error() must render even with a sparse error")
	}
}

func TestOpenAPIErrorFallsBackToHeaderTraceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(TraceIDHeader, "only-in-header")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"err_code":1100300,"message":"系统内部错误"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	err := client.doJSON(t.Context(), http.MethodGet, "/users/@me", nil, nil, openAPICall)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var openAPIErr *OpenAPIError
	if !errors.As(err, &openAPIErr) {
		t.Fatalf("err = %v, want *OpenAPIError", err)
	}
	if openAPIErr.TraceID != "only-in-header" {
		t.Errorf("TraceID = %q, want the header fallback", openAPIErr.TraceID)
	}
	if !openAPIErr.IsServerError() {
		t.Error("HTTP 500 must be a server error")
	}
}

func TestOpenAPIErrorStatusOnlyFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no body here"))
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	err := client.doJSON(t.Context(), http.MethodGet, "/nope", nil, nil, openAPICall)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var openAPIErr *OpenAPIError
	if !errors.As(err, &openAPIErr) {
		t.Fatalf("err = %v, want *OpenAPIError", err)
	}
	if openAPIErr.Code != 0 {
		t.Errorf("Code = %d, want 0 when the body carries none", openAPIErr.Code)
	}
	if !openAPIErr.IsNotFound() {
		t.Error("HTTP 404 must be IsNotFound")
	}
	if openAPIErr.Body == "" {
		t.Error("Body should carry the raw response for debugging")
	}
}
