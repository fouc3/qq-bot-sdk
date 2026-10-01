package qqbotsdk

// Error codes documented by the share link endpoint.
//
// The documentation for POST /v2/generate_url_link disagrees with the common
// table in errcode.go on two numbers:
//
//   - 10001 means "请求参数异常" here, but UnknownAccount in the common table
//   - 10003 means "查询机器人信息异常" here, but UnknownChannel in the common table
//
// The numeric code space is therefore reused per endpoint, so the same value
// must be read in the context of the endpoint that returned it. Neither number
// is defined a second time here; the common constants keep their names, and a
// caller handling this endpoint compares the value directly.
const (
	// ErrRequestHeaderInvalid means the request header was malformed.
	ErrRequestHeaderInvalid OpenAPIErrorCode = 10002
	// ErrUinFromHeaderFailed means the uin could not be read from the
	// Authorization header.
	ErrUinFromHeaderFailed OpenAPIErrorCode = 10044
	// ErrGenerateShareARKFailed means the share ark could not be generated,
	// which the documentation describes as worth retrying.
	ErrGenerateShareARKFailed OpenAPIErrorCode = 11004
)

// shareErrorNames maps the share link codes to their documented text.
var shareErrorNames = map[OpenAPIErrorCode]string{
	ErrRequestHeaderInvalid:   "请求头异常",
	ErrUinFromHeaderFailed:    "从协议头获取uin失败",
	ErrGenerateShareARKFailed: "生成分享ARK失败",
}
