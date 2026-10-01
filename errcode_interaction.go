package qqbotsdk

// Error codes documented by the interaction response endpoint.
//
// These are platform side conditions: an interaction that was already answered,
// one whose window expired, or a token belonging to another app. None of them
// can be judged locally, so they are surfaced as an OpenAPIError for the caller
// to read.
const (
	// ErrInteractionParamInvalid means a request parameter was wrong.
	ErrInteractionParamInvalid OpenAPIErrorCode = 630001
	// ErrInteractionAppIDFailed means the app id could not be read.
	ErrInteractionAppIDFailed OpenAPIErrorCode = 630002
	// ErrInteractionAppIDMismatch means the app id does not own the given
	// interaction id, which happens when the wrong bot token is used.
	ErrInteractionAppIDMismatch OpenAPIErrorCode = 630003
	// ErrInteractionSetDataFailed means the platform could not store the
	// result, and the documentation says to retry later.
	ErrInteractionSetDataFailed OpenAPIErrorCode = 630004
	// ErrInteractionGetDataFailed means the platform could not read the
	// interaction, and the documentation says to retry later.
	ErrInteractionGetDataFailed OpenAPIErrorCode = 630005
	// ErrInteractionHeaderAppIDFailed means the app id was missing from the
	// request header.
	ErrInteractionHeaderAppIDFailed OpenAPIErrorCode = 630006
	// ErrInteractionDataTooLarge means the request body was too large.
	ErrInteractionDataTooLarge OpenAPIErrorCode = 630007
	// ErrInteractionPreprocessFailed means the interaction could not be
	// prepared, which the documentation attributes to the request parameters.
	ErrInteractionPreprocessFailed OpenAPIErrorCode = 630008
)

// interactionErrorNames maps the interaction codes to their documented text.
var interactionErrorNames = map[OpenAPIErrorCode]string{
	ErrInteractionParamInvalid:      "param invalid",
	ErrInteractionAppIDFailed:       "get appid failed",
	ErrInteractionAppIDMismatch:     "appid invalid",
	ErrInteractionSetDataFailed:     "set interaction data failed",
	ErrInteractionGetDataFailed:     "get interaction data failed",
	ErrInteractionHeaderAppIDFailed: "get header appid failed",
	ErrInteractionDataTooLarge:      "data too large",
	ErrInteractionPreprocessFailed:  "interaction preprocess failed",
}
