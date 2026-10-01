package qqbotsdk

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Emoji types used by the reaction endpoints.
const (
	// EmojiTypeSystem is a QQ built-in emoji, identified by a number.
	EmojiTypeSystem = 1
	// EmojiTypeEmoji is a Unicode emoji, identified by the emoji itself.
	EmojiTypeEmoji = 2
)

// ReactionUsers is one page of the users who reacted to a message.
type ReactionUsers struct {
	// Users are the users on this page.
	Users []User `json:"users"`
	// Cookie fetches the next page. Pass it to ReactionUsers again.
	Cookie string `json:"cookie,omitempty"`
	// IsEnd reports whether the last page was reached.
	IsEnd bool `json:"is_end,omitempty"`
}

// reactionPath builds the documented reaction address.
//
// The emoji id is percent-encoded so a Unicode emoji, which is a valid id for
// EmojiTypeEmoji, cannot break the route.
func reactionPath(channelID, messageID string, emojiType int, emojiID string) string {
	return "/channels/" + url.PathEscape(channelID) +
		"/messages/" + url.PathEscape(messageID) +
		"/reactions/" + strconv.Itoa(emojiType) +
		"/" + url.PathEscape(emojiID)
}

// validateReaction rejects an incomplete reaction address locally.
func validateReaction(channelID, messageID, emojiID string, emojiType int) error {
	if channelID == "" || messageID == "" || emojiID == "" {
		return errors.New("qqbotsdk: reaction needs a channel id, a message id and an emoji id")
	}
	if emojiType != EmojiTypeSystem && emojiType != EmojiTypeEmoji {
		return errors.New("qqbotsdk: emoji type must be EmojiTypeSystem or EmojiTypeEmoji")
	}
	return nil
}

// AddReaction adds the bot's reaction to a channel message.
//
// Reactions are only available in channels.
func (c *Client) AddReaction(ctx context.Context, channelID, messageID string, emojiType int, emojiID string) error {
	if err := validateReaction(channelID, messageID, emojiID, emojiType); err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPut, reactionPath(channelID, messageID, emojiType, emojiID), nil, nil, openAPICall)
}

// RemoveReaction removes a reaction the bot added to a channel message.
func (c *Client) RemoveReaction(ctx context.Context, channelID, messageID string, emojiType int, emojiID string) error {
	if err := validateReaction(channelID, messageID, emojiID, emojiType); err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodDelete, reactionPath(channelID, messageID, emojiType, emojiID), nil, nil, openAPICall)
}

// ReactionUsers lists the users who reacted to a channel message.
//
// Pass an empty cookie for the first page. limit is only applied on that first
// request; the documented default is 20 and the maximum is 50. A limit outside
// that range is left to the platform to clamp, except for a negative value,
// which is treated as unset.
func (c *Client) ReactionUsers(ctx context.Context, channelID, messageID string, emojiType int, emojiID, cookie string, limit int) (*ReactionUsers, error) {
	if err := validateReaction(channelID, messageID, emojiID, emojiType); err != nil {
		return nil, err
	}

	query := url.Values{}
	if cookie != "" {
		query.Set("cookie", cookie)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	path := reactionPath(channelID, messageID, emojiType, emojiID)
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out ReactionUsers
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReactionEmoji is a convenience for the documented system emoji ids, so a
// caller does not have to remember which number means which face.
func ReactionEmoji(emojiID string) (int, string, bool) {
	if emojiID == "" || strings.TrimSpace(emojiID) == "" {
		return 0, "", false
	}
	// A numeric id is a QQ built-in emoji; anything else is a Unicode emoji.
	if _, err := strconv.Atoi(emojiID); err == nil {
		return EmojiTypeSystem, emojiID, true
	}
	return EmojiTypeEmoji, emojiID, true
}
