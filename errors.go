package qqbotsdk

import "fmt"

// ErrorCode is a business error code returned in an OpenAPI response body.
type ErrorCode int

// Business error codes documented for the access token endpoint.
const (
	// ErrCodeTooManyRequests means the request was rate limited.
	ErrCodeTooManyRequests ErrorCode = 100001
	// ErrCodeAppIDInvalid means the AppID is invalid, or the bot is
	// banned or deleted.
	ErrCodeAppIDInvalid ErrorCode = 100007
	// ErrCodeInvalidCredential means the AppID or ClientSecret is wrong.
	ErrCodeInvalidCredential ErrorCode = 100016
	// ErrCodeBotNotFound means no bot exists for the AppID.
	ErrCodeBotNotFound ErrorCode = 10004
)

// APIError is a business error reported by the QQ Bot OpenAPI.
//
// The platform returns these with HTTP 200, so they cannot be detected from
// the HTTP status alone. Use errors.As to extract one:
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
