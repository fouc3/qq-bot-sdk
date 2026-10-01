package qqbotsdk

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Rich text element types, as documented for RichObject.
const (
	// RichTypeText is plain text, carried in TextInfo.
	RichTypeText = 1
	// RichTypeAt is a mention, carried in AtInfo.
	RichTypeAt = 2
	// RichTypeURL is a link, carried in URLInfo.
	RichTypeURL = 3
	// RichTypeEmoji is an emoji, carried in EmojiInfo.
	RichTypeEmoji = 4
	// RichTypeChannel is a channel mention, carried in ChannelInfo.
	RichTypeChannel = 5
	// RichTypeVideo is a video.
	RichTypeVideo = 10
	// RichTypeImage is an image.
	RichTypeImage = 11
)

// Mention types, as documented for AtInfo.
const (
	// AtTypeExplicitUser mentions one user.
	AtTypeExplicitUser = 1
	// AtTypeRoleGroup mentions everyone in a role group.
	AtTypeRoleGroup = 2
	// AtTypeGuild mentions everyone in the guild.
	AtTypeGuild = 3
)

// Forum audit types, as documented for ForumAuditResult.
const (
	// ForumAuditPublishThread audits a thread.
	ForumAuditPublishThread = 1
	// ForumAuditPublishPost audits a post.
	ForumAuditPublishPost = 2
	// ForumAuditPublishReply audits a reply.
	ForumAuditPublishReply = 3
)

// Forum audit results.
const (
	// ForumAuditSuccess means the content passed.
	ForumAuditSuccess = 0
	// ForumAuditFailure means the content was rejected.
	ForumAuditFailure = 1
)

// RichTextInfo is the plain text of a rich text element.
type RichTextInfo struct {
	// Text is the text.
	Text string `json:"text,omitempty"`
}

// RichAtUserInfo describes a mentioned user.
type RichAtUserInfo struct {
	// ID is the id group id.
	ID string `json:"id,omitempty"`
	// Nick is the nickname.
	Nick string `json:"nick,omitempty"`
}

// RichAtRoleInfo describes a mentioned role group.
type RichAtRoleInfo struct {
	// RoleID is the role group id.
	RoleID uint64 `json:"role_id,omitempty"`
	// Name is the role group name.
	Name string `json:"name,omitempty"`
	// Color is the colour value.
	Color uint32 `json:"color,omitempty"`
}

// RichAtGuildInfo describes a mention of everyone in the guild.
type RichAtGuildInfo struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// GuildName is the guild name.
	GuildName string `json:"guild_name,omitempty"`
}

// RichAtInfo is a mention element.
type RichAtInfo struct {
	// Type is one of the AtType constants.
	Type int `json:"type,omitempty"`
	// UserInfo is set when Type is AtTypeExplicitUser.
	UserInfo *RichAtUserInfo `json:"user_info,omitempty"`
	// RoleInfo is set when Type is AtTypeRoleGroup.
	RoleInfo *RichAtRoleInfo `json:"role_info,omitempty"`
	// GuildInfo is set when Type is AtTypeGuild.
	GuildInfo *RichAtGuildInfo `json:"guild_info,omitempty"`
}

// RichURLInfo is a link element.
type RichURLInfo struct {
	// URL is the address.
	URL string `json:"url,omitempty"`
	// DisplayText is the shown text.
	DisplayText string `json:"display_text,omitempty"`
}

// RichEmojiInfo is an emoji element.
type RichEmojiInfo struct {
	// ID is the emoji id.
	//
	// The field table types it as a string while the examples write a number,
	// so both decode; see UnmarshalJSON.
	ID string `json:"id,omitempty"`
	// Type is the emoji type.
	Type string `json:"type,omitempty"`
	// Name is the emoji name.
	Name string `json:"name,omitempty"`
	// URL is the emoji image.
	URL string `json:"url,omitempty"`
}

// UnmarshalJSON tolerates a numeric emoji id, which the examples use against
// the field table.
func (e *RichEmojiInfo) UnmarshalJSON(data []byte) error {
	type plain RichEmojiInfo
	var wire struct {
		plain
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = RichEmojiInfo(wire.plain)
	e.ID = looseString(wire.ID)
	return nil
}

// RichChannelInfo is a channel mention element.
//
// It is named apart from ChannelInfo, which describes a channel object.
type RichChannelInfo struct {
	// ChannelID is the mentioned channel id.
	ChannelID uint64 `json:"channel_id,omitempty"`
	// ChannelName is the mentioned channel name.
	ChannelName string `json:"channel_name,omitempty"`
}

// RichObject is one rich text element of a forum thread, post or reply.
//
// Which field is read depends on Type.
type RichObject struct {
	// Type is one of the RichType constants.
	Type int `json:"type,omitempty"`
	// TextInfo is set when Type is RichTypeText.
	TextInfo *RichTextInfo `json:"text_info,omitempty"`
	// AtInfo is set when Type is RichTypeAt.
	AtInfo *RichAtInfo `json:"at_info,omitempty"`
	// URLInfo is set when Type is RichTypeURL.
	URLInfo *RichURLInfo `json:"url_info,omitempty"`
	// EmojiInfo is set when Type is RichTypeEmoji.
	EmojiInfo *RichEmojiInfo `json:"emoji_info,omitempty"`
	// ChannelInfo is set when Type is RichTypeChannel.
	ChannelInfo *RichChannelInfo `json:"channel_info,omitempty"`
}

// PlainText flattens the element into plain text.
func (o RichObject) PlainText() string {
	switch {
	case o.TextInfo != nil:
		return o.TextInfo.Text
	case o.URLInfo != nil:
		return o.URLInfo.DisplayText
	case o.EmojiInfo != nil:
		return o.EmojiInfo.Name
	case o.ChannelInfo != nil:
		// The documented channel name already carries its leading #, so one
		// is not added here.
		return o.ChannelInfo.ChannelName
	case o.AtInfo != nil:
		switch {
		case o.AtInfo.UserInfo != nil:
			return "@" + o.AtInfo.UserInfo.Nick
		case o.AtInfo.RoleInfo != nil:
			return "@" + o.AtInfo.RoleInfo.Name
		case o.AtInfo.GuildInfo != nil:
			return "@" + o.AtInfo.GuildInfo.GuildName
		}
	}
	return ""
}

// RichTextValue holds the rich text of a forum thread, post or reply.
//
// The documentation's field tables type these fields as string, while every
// example shows an array of rich text objects:
//
//	"title": [{"type": 1, "text_info": {"text": "Test"}}]
//
// This type accepts either form, so a payload is never rejected over the
// disagreement. Use PlainText for the flattened text.
type RichTextValue struct {
	// Elements are the rich text pieces, when the payload carried an array.
	Elements []RichObject
	// Text is the plain text, when the payload carried a string.
	Text string
}

// UnmarshalJSON accepts both the string and the object array form.
func (r *RichTextValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		return json.Unmarshal(trimmed, &r.Text)
	}
	return json.Unmarshal(trimmed, &r.Elements)
}

// MarshalJSON emits the array form, which is the shape the examples show.
func (r RichTextValue) MarshalJSON() ([]byte, error) {
	if len(r.Elements) > 0 {
		return json.Marshal(r.Elements)
	}
	return json.Marshal(r.Text)
}

// PlainText flattens the value into plain text.
func (r RichTextValue) PlainText() string {
	if len(r.Elements) == 0 {
		return r.Text
	}
	var b strings.Builder
	for _, element := range r.Elements {
		b.WriteString(element.PlainText())
	}
	return b.String()
}

// ThreadInfo is the content of a forum thread.
type ThreadInfo struct {
	// ThreadID is the thread id.
	ThreadID string `json:"thread_id,omitempty"`
	// Title is the thread title.
	Title RichTextValue `json:"title,omitempty"`
	// Content is the thread body.
	Content RichTextValue `json:"content,omitempty"`
	// DateTime is the posting time in ISO8601.
	DateTime string `json:"date_time,omitempty"`
}

// PostInfo is the content of a forum post.
type PostInfo struct {
	// ThreadID is the thread it belongs to.
	ThreadID string `json:"thread_id,omitempty"`
	// PostID is the post id.
	PostID string `json:"post_id,omitempty"`
	// Content is the post body.
	Content RichTextValue `json:"content,omitempty"`
	// DateTime is the posting time.
	DateTime string `json:"date_time,omitempty"`
}

// ReplyInfo is the content of a forum reply.
type ReplyInfo struct {
	// ThreadID is the thread it belongs to.
	ThreadID string `json:"thread_id,omitempty"`
	// PostID is the post it answers.
	PostID string `json:"post_id,omitempty"`
	// ReplyID is the reply id.
	ReplyID string `json:"reply_id,omitempty"`
	// Content is the reply body.
	Content RichTextValue `json:"content,omitempty"`
	// DateTime is the posting time.
	DateTime string `json:"date_time,omitempty"`
}

// ForumThreadEvent is the body of the three thread events.
type ForumThreadEvent struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AuthorID is the author's id.
	AuthorID string `json:"author_id,omitempty"`
	// ThreadInfo is the thread content.
	ThreadInfo *ThreadInfo `json:"thread_info,omitempty"`
}

// ForumPostEvent is the body of FORUM_POST_CREATE and FORUM_POST_DELETE.
type ForumPostEvent struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AuthorID is the author's id.
	AuthorID string `json:"author_id,omitempty"`
	// PostInfo is the post content.
	PostInfo *PostInfo `json:"post_info,omitempty"`
}

// ForumReplyEvent is the body of FORUM_REPLY_CREATE and FORUM_REPLY_DELETE.
type ForumReplyEvent struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AuthorID is the author's id.
	AuthorID string `json:"author_id,omitempty"`
	// ReplyInfo is the reply content.
	ReplyInfo *ReplyInfo `json:"reply_info,omitempty"`
}

// ForumAuditResult is the body of FORUM_PUBLISH_AUDIT_RESULT.
type ForumAuditResult struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AuthorID is the author's id.
	AuthorID string `json:"author_id,omitempty"`
	// ThreadID is the thread id.
	ThreadID string `json:"thread_id,omitempty"`
	// PostID is the post id.
	PostID string `json:"post_id,omitempty"`
	// ReplyID is the reply id.
	ReplyID string `json:"reply_id,omitempty"`
	// Type is one of the ForumAudit constants.
	Type uint32 `json:"type,omitempty"`
	// Result is ForumAuditSuccess or ForumAuditFailure.
	Result uint32 `json:"result,omitempty"`
	// ErrMsg explains a failure when Result is not ForumAuditSuccess.
	ErrMsg string `json:"err_msg,omitempty"`
}

// looseString decodes a JSON value into a string, accepting both the quoted and
// the bare numeric form.
//
// The forum documentation's field tables type its ids as string while its
// examples write them as numbers, so both must decode rather than failing the
// whole event.
func looseString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// UnmarshalJSON tolerates a numeric guild_id, channel_id or author_id.
func (e *ForumThreadEvent) UnmarshalJSON(data []byte) error {
	type plain ForumThreadEvent
	var wire struct {
		plain
		GuildID   json.RawMessage `json:"guild_id"`
		ChannelID json.RawMessage `json:"channel_id"`
		AuthorID  json.RawMessage `json:"author_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = ForumThreadEvent(wire.plain)
	e.GuildID = looseString(wire.GuildID)
	e.ChannelID = looseString(wire.ChannelID)
	e.AuthorID = looseString(wire.AuthorID)
	return nil
}

// UnmarshalJSON tolerates a numeric guild_id, channel_id or author_id.
func (e *ForumPostEvent) UnmarshalJSON(data []byte) error {
	type plain ForumPostEvent
	var wire struct {
		plain
		GuildID   json.RawMessage `json:"guild_id"`
		ChannelID json.RawMessage `json:"channel_id"`
		AuthorID  json.RawMessage `json:"author_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = ForumPostEvent(wire.plain)
	e.GuildID = looseString(wire.GuildID)
	e.ChannelID = looseString(wire.ChannelID)
	e.AuthorID = looseString(wire.AuthorID)
	return nil
}

// UnmarshalJSON tolerates a numeric guild_id, channel_id or author_id.
func (e *ForumReplyEvent) UnmarshalJSON(data []byte) error {
	type plain ForumReplyEvent
	var wire struct {
		plain
		GuildID   json.RawMessage `json:"guild_id"`
		ChannelID json.RawMessage `json:"channel_id"`
		AuthorID  json.RawMessage `json:"author_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = ForumReplyEvent(wire.plain)
	e.GuildID = looseString(wire.GuildID)
	e.ChannelID = looseString(wire.ChannelID)
	e.AuthorID = looseString(wire.AuthorID)
	return nil
}

// UnmarshalJSON tolerates a numeric guild_id, channel_id or author_id.
func (e *ForumAuditResult) UnmarshalJSON(data []byte) error {
	type plain ForumAuditResult
	var wire struct {
		plain
		GuildID   json.RawMessage `json:"guild_id"`
		ChannelID json.RawMessage `json:"channel_id"`
		AuthorID  json.RawMessage `json:"author_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*e = ForumAuditResult(wire.plain)
	e.GuildID = looseString(wire.GuildID)
	e.ChannelID = looseString(wire.ChannelID)
	e.AuthorID = looseString(wire.AuthorID)
	return nil
}
