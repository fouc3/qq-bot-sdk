package qqbotsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode is a business error code returned by the access token endpoint in
// the "code" field of an otherwise HTTP 200 response.
type ErrorCode int

// Business error codes documented for the access token endpoint.
const (
	// ErrCodeTooManyRequests means the request was rate limited.
	ErrCodeTooManyRequests ErrorCode = 100001
	// ErrCodeAppIDInvalid means the AppID is invalid, or the bot is banned or
	// deleted.
	ErrCodeAppIDInvalid ErrorCode = 100007
	// ErrCodeInvalidCredential means the AppID or ClientSecret is wrong.
	ErrCodeInvalidCredential ErrorCode = 100016
	// ErrCodeBotNotFound means no bot exists for the AppID.
	ErrCodeBotNotFound ErrorCode = 10004
)

// APIError is a business error reported by the access token endpoint.
//
// The platform returns these with HTTP 200, so they cannot be detected from the
// HTTP status alone. Use errors.As to extract one:
//
//	var apiErr *qqbotsdk.APIError
//	if errors.As(err, &apiErr) && apiErr.Code == qqbotsdk.ErrCodeInvalidCredential {
//		// ...
//	}
type APIError struct {
	// Code is the business error code.
	Code ErrorCode
	// Message is a human readable hint. The platform may change it at any
	// time, so branch on Code rather than on this text.
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("qqbotsdk: api error %d: %s", e.Code, e.Message)
}

// OpenAPIError is a failed OpenAPI call, carrying everything needed to report
// or debug it: where the request went, what came back, and the platform's
// error code and trace id.
//
// The platform signals failure two ways, and both are captured here:
//
//   - a non-success HTTP status (401, 404, 405, 429, 500, 504, ...), and
//   - an err_code in the JSON body, which may accompany HTTP 200.
//
// Branch on Code rather than Message: the documentation states that message
// text may change at any time.
type OpenAPIError struct {
	// Method is the HTTP method of the failed request.
	Method string
	// URL is the full request address.
	URL string
	// StatusCode is the HTTP status code. Zero when the request never
	// reached the platform.
	StatusCode int
	// Code is the err_code from the response body. Zero when the body carried
	// none, in which case StatusCode describes the failure.
	Code OpenAPIErrorCode
	// Message is the platform message, kept for human inspection only.
	Message string
	// TraceID is the platform trace id, taken from the trace_id body field or
	// the X-Tps-trace-ID response header. It is the value to hand to platform
	// support when a problem cannot be diagnosed locally.
	TraceID string
	// Body is a truncated copy of the raw response body, for debugging.
	Body string
}

// Error renders the failure with request, status, code and trace information.
func (e *OpenAPIError) Error() string {
	var b strings.Builder
	b.WriteString("qqbotsdk: ")
	if e.Method != "" {
		b.WriteString(e.Method)
		b.WriteByte(' ')
	}
	b.WriteString(e.URL)
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, ": http %d", e.StatusCode)
	}
	if e.Code != 0 {
		fmt.Fprintf(&b, ": err_code %d", e.Code)
		if name := e.Code.String(); name != "" {
			fmt.Fprintf(&b, " (%s)", name)
		}
	}
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	}
	if e.TraceID != "" {
		fmt.Fprintf(&b, " [trace_id=%s]", e.TraceID)
	}
	return b.String()
}

// HasCode reports whether the failure carries the given err_code.
//
// It is named HasCode rather than Is so it cannot be confused with the
// errors.Is convention, which takes an error rather than a code.
func (e *OpenAPIError) HasCode(code OpenAPIErrorCode) bool {
	return e.Code == code
}

// IsStatus reports whether the failure carries the given HTTP status code.
func (e *OpenAPIError) IsStatus(status int) bool {
	return e.StatusCode == status
}

// IsAuthFailure reports an authentication failure (HTTP 401), or the err_code
// values that mean the credential itself was rejected.
func (e *OpenAPIError) IsAuthFailure() bool {
	switch {
	case e.StatusCode == http.StatusUnauthorized:
		return true
	case e.Code == ErrWrongToken, e.Code == ErrCheckTokenNotPass,
		e.Code == ErrWrongAppID, e.Code == ErrMissingAppID, e.Code == ErrNoAppID:
		return true
	}
	return false
}

// IsNotFound reports that the requested API or resource was not found.
func (e *OpenAPIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsMethodNotAllowed reports an HTTP method that the endpoint rejects.
func (e *OpenAPIError) IsMethodNotAllowed() bool {
	return e.StatusCode == http.StatusMethodNotAllowed
}

// IsRateLimited reports that the call hit a frequency limit, either at the
// HTTP layer (429) or as a business code.
func (e *OpenAPIError) IsRateLimited() bool {
	switch e.Code {
	case ErrChannelHitWriteRateLimit, ErrSafeMessageRateLimited, ErrTriggerChannelRateLimit:
		return true
	}
	return e.StatusCode == http.StatusTooManyRequests
}

// IsServerError reports a platform-side failure (HTTP 500 or 504).
func (e *OpenAPIError) IsServerError() bool {
	return e.StatusCode == http.StatusInternalServerError ||
		e.StatusCode == http.StatusGatewayTimeout
}

// IsAsyncAccepted reports HTTP 201 or 202: the operation was accepted for
// asynchronous processing.
//
// The documentation notes that these responses still carry an error body,
// which is why they surface as an OpenAPIError. Treat one as pending rather
// than as a hard failure, for example ErrPushMessageAsyncOK.
func (e *OpenAPIError) IsAsyncAccepted() bool {
	return e.StatusCode == http.StatusCreated || e.StatusCode == http.StatusAccepted
}

// Retryable reports whether retrying the same request may succeed.
//
// It covers the transport-level failures the documentation marks as worth one
// retry, plus rate limiting. The documentation warns that the
// "check ... failed" system errors may be retried at most once, so callers
// should not loop on them indefinitely.
func (e *OpenAPIError) Retryable() bool {
	if e.IsRateLimited() || e.IsServerError() {
		return true
	}
	switch e.Code {
	case ErrCheckAdminFailed, ErrCheckAppPrivilegeFailed, ErrCheckGuildAuth,
		ErrCheckTokenFailed, ErrGetMessageFailed, ErrRetractMessageFailed,
		ErrGetChannelFailed:
		return true
	}
	return false
}

// errorBody is the error envelope shared by OpenAPI responses.
type errorBody struct {
	ErrCode OpenAPIErrorCode `json:"err_code"`
	Message string           `json:"message"`
	TraceID string           `json:"trace_id"`
}

// openAPIError inspects a finished OpenAPI response and returns an
// *OpenAPIError when the call failed, or nil when it succeeded.
//
// The response is reported as failed when either the HTTP status is not a
// success status, or the body carries a non-zero err_code. A body that cannot
// be parsed is only treated as an error when the status itself is a failure,
// so an unparsable success body is left for the caller's decoder to report.
func openAPIError(method, url string, resp *http.Response, body []byte) error {
	var parsed errorBody
	decodeErr := json.Unmarshal(body, &parsed)

	statusFailed := resp.StatusCode < 200 || resp.StatusCode > 299
	codeFailed := decodeErr == nil && parsed.ErrCode != 0

	if !statusFailed && !codeFailed {
		return nil
	}
	return &OpenAPIError{
		Method:     method,
		URL:        url,
		StatusCode: resp.StatusCode,
		Code:       parsed.ErrCode,
		Message:    parsed.Message,
		TraceID:    firstNonEmpty(parsed.TraceID, resp.Header.Get(TraceIDHeader)),
		Body:       snippet(body),
	}
}

// firstNonEmpty returns the first argument that is not empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// IsOpenAPIError reports whether err is an *OpenAPIError carrying the given
// err_code. It is a convenience wrapper around errors.As.
func IsOpenAPIError(err error, code OpenAPIErrorCode) bool {
	var openAPIErr *OpenAPIError
	return errors.As(err, &openAPIErr) && openAPIErr.Code == code
}
