package qqbotsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Paths of the endpoints that describe the bot itself.
const (
	usersMePath       = "/users/@me"
	usersMeGuildsPath = "/users/@me/guilds"
)

// maxGuildPageSize is the documented maximum for one page of guilds.
const maxGuildPageSize = 100

// BotInfo is the bot's own account details.
type BotInfo struct {
	// ID is the bot's user id.
	ID string `json:"id"`
	// Username is the bot's display name.
	Username string `json:"username"`
	// Avatar is the avatar URL.
	Avatar string `json:"avatar"`
	// Bot reports whether the account is a bot.
	//
	// It is a pointer because the live platform does not send the field at
	// all: calling GET /users/@me for the bot account returns id, username,
	// avatar, share_url and welcome_msg, but no bot key, even though the
	// documented example shows "bot": true. A plain bool would therefore
	// report a bot as "not a bot". Nil means the platform did not say.
	Bot *bool `json:"bot,omitempty"`
	// UnionOpenID is the cross-application user openid. The platform returns
	// it only after a special application and configuration; it was absent
	// from the live response.
	UnionOpenID string `json:"union_openid,omitempty"`
	// UnionUserAccount is the cross-application user account, under the same
	// restriction as UnionOpenID.
	UnionUserAccount string `json:"union_user_account,omitempty"`
	// ShareURL is the bot's share link. It appears in the documented response
	// example but not in the field table. The live platform does return it.
	ShareURL string `json:"share_url,omitempty"`
	// WelcomeMsg is the bot's welcome message, with the same caveat as
	// ShareURL. The live platform returns the key, empty when unset.
	WelcomeMsg string `json:"welcome_msg,omitempty"`
}

// IsBot reports whether the platform said the account is a bot, and whether it
// said anything at all.
func (b *BotInfo) IsBot() (value, reported bool) {
	if b == nil || b.Bot == nil {
		return false, false
	}
	return *b.Bot, true
}

// GuildInfo describes one guild.
//
// It is used both by the guild list endpoint and by the guild create, update
// and delete events. The list endpoint returns Owner; the events return
// OpUserID instead, so each is absent in the other's payload.
type GuildInfo struct {
	// ID is the guild id.
	ID string `json:"id"`
	// Name is the guild name.
	Name string `json:"name"`
	// Icon is the guild avatar URL.
	Icon string `json:"icon"`
	// OwnerID is the id of the guild creator.
	OwnerID string `json:"owner_id"`
	// Owner reports whether the bot owns the guild, in the list response.
	Owner bool `json:"owner"`
	// JoinedAt is when the bot joined, in ISO8601.
	JoinedAt string `json:"joined_at"`
	// MemberCount is the current number of members.
	MemberCount int `json:"member_count"`
	// MaxMembers is the member limit of the guild.
	MaxMembers int `json:"max_members"`
	// Description is the guild description.
	Description string `json:"description"`
	// OpUserID is the id of the operator, in a guild event.
	OpUserID string `json:"op_user_id,omitempty"`
}

// guildListResponse accepts both shapes the documentation shows for the guild
// list endpoint: the field table documents an object with a guilds field, while
// the response example is a bare array.
type guildListResponse struct {
	Guilds []GuildInfo
}

// UnmarshalJSON decodes either the wrapped object or the bare array form.
func (g *guildListResponse) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		g.Guilds = nil
		return nil
	}
	if trimmed[0] == '[' {
		return json.Unmarshal(trimmed, &g.Guilds)
	}
	var wrapped struct {
		Guilds []GuildInfo `json:"guilds"`
	}
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return fmt.Errorf("qqbotsdk: decode guild list: %w", err)
	}
	g.Guilds = wrapped.Guilds
	return nil
}

// GetBotInfo returns the bot's own details.
//
// It is the documented GET /users/@me endpoint.
func (c *Client) GetBotInfo(ctx context.Context) (*BotInfo, error) {
	var out BotInfo
	if err := c.doJSON(ctx, http.MethodGet, usersMePath, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetJoinedGuilds lists the guilds the bot has joined.
//
// after and before are guild ids used as pagination cursors; pass empty strings
// to omit them. The documentation notes that before wins when both are set, and
// that a page is read in reverse order when before is used. limit defaults to
// 100 and is capped at 100.
func (c *Client) GetJoinedGuilds(ctx context.Context, after, before string, limit int) ([]GuildInfo, error) {
	query := url.Values{}
	if before != "" {
		query.Set("before", before)
	}
	if after != "" {
		query.Set("after", after)
	}
	if limit > 0 {
		if limit > maxGuildPageSize {
			limit = maxGuildPageSize
		}
		query.Set("limit", strconv.Itoa(limit))
	}

	path := usersMeGuildsPath
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var out guildListResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return out.Guilds, nil
}
