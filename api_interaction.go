package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// InteractionCode is the result reported when answering an interaction.
//
// The zero value is InteractionCodeSuccess, which is also the documented
// default, so the ordinary "handled it" answer needs no extra thought.
type InteractionCode int

// The documented results of answering an interaction.
const (
	// InteractionCodeSuccess reports that the action succeeded.
	InteractionCodeSuccess InteractionCode = 0
	// InteractionCodeFailed reports that the action failed.
	InteractionCodeFailed InteractionCode = 1
	// InteractionCodeTooFrequent reports that the user acted too often.
	InteractionCodeTooFrequent InteractionCode = 2
	// InteractionCodeDuplicate reports that the action was already done.
	InteractionCodeDuplicate InteractionCode = 3
	// InteractionCodeNoPermission reports that the user may not do it.
	InteractionCodeNoPermission InteractionCode = 4
	// InteractionCodeAdminOnly reports that only an administrator may do it.
	InteractionCodeAdminOnly InteractionCode = 5
)

// Validate reports whether the code is one the platform documents.
//
// A code outside the range is refused here rather than sent: the platform would
// answer 630001, and one round trip is cheaper to avoid than to diagnose. A
// code inside the range that the platform still rejects, such as one for an
// interaction that expired, comes back as an OpenAPIError.
func (c InteractionCode) Validate() error {
	if c < InteractionCodeSuccess || c > InteractionCodeAdminOnly {
		return fmt.Errorf("qqbotsdk: interaction code %d is not one of 0 success, "+
			"1 failed, 2 too frequent, 3 duplicate, 4 no permission or 5 admin only", int(c))
	}
	return nil
}

// String returns the documented meaning of the code.
func (c InteractionCode) String() string {
	switch c {
	case InteractionCodeSuccess:
		return "success"
	case InteractionCodeFailed:
		return "failed"
	case InteractionCodeTooFrequent:
		return "too frequent"
	case InteractionCodeDuplicate:
		return "duplicate"
	case InteractionCodeNoPermission:
		return "no permission"
	case InteractionCodeAdminOnly:
		return "admin only"
	}
	return fmt.Sprintf("InteractionCode(%d)", int(c))
}

// interactionIDPrefix is a prefix the interaction id is sometimes written with.
// The endpoint takes the bare id; the documentation calls this out explicitly.
const interactionIDPrefix = "INTERACTION_CREATE:"

// normalizeInteractionID removes the optional prefix and surrounding space.
func normalizeInteractionID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, interactionIDPrefix)
	return strings.TrimSpace(id)
}

// RespondInteraction answers an INTERACTION_CREATE event.
//
// The event arrives over the websocket or as a webhook callback, but the answer
// is a separate HTTPS call to this endpoint rather than a reply on that
// channel: the webhook acknowledgement only tells the platform the push was
// received and carries no result, so an interaction still needs this call or
// the user's client keeps showing a loading state until it times out.
//
// Only a message button (InteractionInlineKeyboard) and a single chat menu
// (InteractionCallbackCommand) need an answer; the platform accepts a call for
// the other types without complaining. InteractionCreateData.NeedsResponse
// reports which is which.
//
// The documentation says an interaction id may be answered only once, and that
// it expires. Production showed the "only once" half is not enforced by an
// error: a second answer to the same id was accepted, so presumably only the
// first one takes effect. Answering an expired id is expected to fail with one
// of the 63000x codes, which is the platform's business and not something that
// can be judged here.
func (c *Client) RespondInteraction(ctx context.Context, interactionID string, code InteractionCode) error {
	id := normalizeInteractionID(interactionID)
	if id == "" {
		return errors.New("qqbotsdk: RespondInteraction needs an interaction id")
	}
	if err := code.Validate(); err != nil {
		return err
	}

	payload := struct {
		Code InteractionCode `json:"code"`
	}{Code: code}

	return c.doJSON(ctx, http.MethodPut, "/interactions/"+url.PathEscape(id), payload, nil, openAPICall)
}
