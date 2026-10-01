package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"
)

// generateURLPath is the share link generator endpoint.
const generateURLPath = "/v2/generate_url_link"

// maxCallbackDataRunes is the documented limit for callback_data.
const maxCallbackDataRunes = 32

// GenerateShareLink creates a share link that invites users to add the bot as a
// friend.
//
// callbackData is passed back to the bot when a user adds it through the link,
// and is limited to 32 characters. Pass an empty string to omit it.
//
// It returns the generated URL, which is the data.url field of the response.
func (c *Client) GenerateShareLink(ctx context.Context, callbackData string) (string, error) {
	// The platform rejects an over-long value, so catching it here saves a
	// round trip and reports the limit in the error.
	if utf8.RuneCountInString(callbackData) > maxCallbackDataRunes {
		return "", fmt.Errorf("qqbotsdk: callback_data is longer than %d characters", maxCallbackDataRunes)
	}

	payload := struct {
		CallbackData string `json:"callback_data,omitempty"`
	}{CallbackData: callbackData}

	var out struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, generateURLPath, payload, &out, openAPICall); err != nil {
		return "", err
	}
	if out.Data.URL == "" {
		return "", errors.New("qqbotsdk: " + generateURLPath + ": empty url in response")
	}
	return out.Data.URL, nil
}
