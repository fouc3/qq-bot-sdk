package qqbotsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
)

// MsgType selects which field of a message request carries the content.
//
// The documentation defines it per endpoint; these are the values accepted by
// the single-chat and group endpoints.
const (
	// MsgTypeText carries content.
	MsgTypeText = 0
	// MsgTypeMarkdown carries markdown.
	MsgTypeMarkdown = 2
	// MsgTypeInputNotify carries input_notify, the "typing" state.
	MsgTypeInputNotify = 6
	// MsgTypeMedia carries media, a previously uploaded file.
	MsgTypeMedia = 7
)

// Keyboard render styles.
const (
	KeyboardStyleGrey     = 0
	KeyboardStyleBlue     = 1
	KeyboardStyleRed      = 3
	KeyboardStyleBlueFill = 4
)

// Button action types.
const (
	// ActionTypeLink opens an http or mini program link.
	ActionTypeLink = 0
	// ActionTypeCallback reports the click back to the bot.
	ActionTypeCallback = 1
	// ActionTypeCommand inserts the data into the composer for the user to
	// send, rather than reporting a click.
	//
	// Production confirmed it produces no INTERACTION_CREATE event, so nothing
	// needs answering: the button filled the composer with its data and the
	// person sent it as an ordinary message.
	ActionTypeCommand = 2
)

// Button permission types.
const (
	PermissionTypeSpecified = 0
	PermissionTypeAdmin     = 1
	PermissionTypeEveryone  = 2
)

// Stream message constants.
const (
	// StreamInputGenerating marks a chunk as still being generated.
	StreamInputGenerating = 1
	// StreamInputFinished marks the last chunk.
	StreamInputFinished = 10

	// StreamContentText streams plain text.
	StreamContentText = "text"
	// StreamContentMarkdown streams markdown.
	StreamContentMarkdown = "markdown"

	// StreamInputAppend appends content_raw to the pending text.
	StreamInputAppend = "append"
	// StreamInputReplace treats content_raw as the whole text so far.
	StreamInputReplace = "replace"
)

// Message is the request body of the single-chat and group send endpoints.
//
// Which field is read depends on MsgType. Note that the platform validates the
// combination, for example that markdown and content are not both set.
type Message struct {
	// MsgType selects the content field. The zero value is plain text.
	MsgType int `json:"msg_type,omitempty"`
	// Content is the plain text body, used when MsgType is MsgTypeText.
	Content string `json:"content,omitempty"`
	// Markdown is the markdown body, used when MsgType is MsgTypeMarkdown.
	Markdown *MessageMarkdown `json:"markdown,omitempty"`
	// Keyboard is the message button keyboard, either a template id or a
	// custom layout.
	Keyboard *Keyboard `json:"keyboard,omitempty"`
	// MsgID replies to the message that carried it, making this a passive
	// reply. It comes from the event body id.
	MsgID string `json:"msg_id,omitempty"`
	// EventID replies to an event instead of a message. Mutually exclusive
	// with MsgID.
	EventID string `json:"event_id,omitempty"`
	// MsgSeq numbers replies to the same MsgID so they are not deduplicated.
	// The platform defaults it to 1.
	MsgSeq int `json:"msg_seq,omitempty"`
	// Media sends an uploaded file, used when MsgType is MsgTypeMedia.
	Media *MediaInfo `json:"media,omitempty"`
	// MessageReference shows this message as a quote of another one.
	MessageReference *MessageReference `json:"message_reference,omitempty"`
	// IsWakeup marks an interaction recall message. Mutually exclusive with
	// MsgID and EventID.
	IsWakeup bool `json:"is_wakeup,omitempty"`
	// InputNotify reports the typing state, used when MsgType is
	// MsgTypeInputNotify.
	InputNotify *InputNotify `json:"input_notify,omitempty"`
}

// MessageMarkdown is the markdown body of a message.
type MessageMarkdown struct {
	// TemplateID refers to a platform template. Deprecated by the platform.
	TemplateID int `json:"template_id,omitempty"`
	// Content is raw markdown.
	Content string `json:"content,omitempty"`
	// CustomTemplateID refers to a custom template. Deprecated.
	CustomTemplateID string `json:"custom_template_id,omitempty"`
	// ForceVerifyImageResource fails the send when an image cannot be
	// transferred, instead of sending anyway.
	ForceVerifyImageResource bool `json:"force_verify_image_resource,omitempty"`
}

// Keyboard is a message button keyboard.
//
// Either ID selects a platform template, or Content defines a custom layout.
//
// A keyboard hangs off a markdown message: the documentation opens with
// "在 markdown 消息的基础上，支持消息最底部挂载按钮". Sending one with a plain
// text message is accepted by the platform but the buttons are dropped with no
// error at all, which is why Message.Validate refuses that combination before
// the request is made.
type Keyboard struct {
	ID      string           `json:"id,omitempty"`
	Content *KeyboardContent `json:"content,omitempty"`
}

// KeyboardContent is a custom keyboard layout.
type KeyboardContent struct {
	Rows []Row `json:"rows,omitempty"`
}

// Row is one row of buttons, ordered left to right.
type Row struct {
	Buttons []Button `json:"buttons,omitempty"`
}

// Button is one message button.
type Button struct {
	// ID is unique within the keyboard.
	ID string `json:"id,omitempty"`
	// RenderData controls how the button looks.
	RenderData *RenderData `json:"render_data,omitempty"`
	// Action controls what a click does.
	Action *Action `json:"action,omitempty"`
	// GroupID greys out the other buttons of the group once one is used, so
	// the rest cannot be clicked. Only meaningful when Action.Type is
	// ActionTypeCallback.
	//
	// Buttons without a group are independent: production showed that using one
	// leaves its ungrouped neighbours clickable.
	GroupID string `json:"group_id,omitempty"`
}

// RenderData is the appearance of a button.
type RenderData struct {
	// Label is the button text, at most 10 characters.
	Label string `json:"label,omitempty"`
	// VisitedLabel is the text shown after a click.
	//
	// It is also the state the button stays in: production showed that once a
	// callback button has been clicked, that message's button can no longer be
	// clicked at all. To offer the action again, send a new message carrying a
	// new keyboard rather than expecting the old button to work twice.
	VisitedLabel string `json:"visited_label,omitempty"`
	// Style is one of the KeyboardStyle values.
	Style int `json:"style,omitempty"`
}

// Action is the behaviour of a button click.
type Action struct {
	// Type is one of the ActionType values.
	Type int `json:"type,omitempty"`
	// Permission restricts who may use the button.
	Permission *Permission `json:"permission,omitempty"`
	// Data is the callback payload, required for callback and command
	// buttons.
	Data string `json:"data,omitempty"`
	// ClickLimit is deprecated by the platform.
	ClickLimit int `json:"click_limit,omitempty"`
	// UnsupportTips is shown on clients too old to support the button.
	UnsupportTips string `json:"unsupport_tips,omitempty"`
	// Enter sends Data immediately on click. Single chat only.
	Enter bool `json:"enter,omitempty"`
	// Reply makes the command quote this message. Single chat only.
	Reply bool `json:"reply,omitempty"`
	// Anchor is only meaningful for command buttons.
	Anchor int `json:"anchor,omitempty"`
	// Modal asks the user to confirm before the action runs.
	Modal *Modal `json:"modal,omitempty"`
}

// Permission restricts who may click a button.
type Permission struct {
	// Type is one of the PermissionType values.
	Type int `json:"type,omitempty"`
	// SpecifyUserIDs lists the allowed users.
	SpecifyUserIDs []string `json:"specify_user_ids,omitempty"`
	// SpecifyRoleIDs lists the allowed roles. Channel only.
	SpecifyRoleIDs []string `json:"specify_role_ids,omitempty"`
}

// Modal is a confirmation dialog shown before a button action runs.
type Modal struct {
	// Content is the prompt, at most 40 characters and without URLs.
	Content string `json:"content,omitempty"`
	// ConfirmText is the confirm button label, at most 4 characters.
	ConfirmText string `json:"confirm_text,omitempty"`
	// CancelText is the cancel button label, at most 4 characters.
	CancelText string `json:"cancel_text,omitempty"`
}

// MediaInfo sends a file that was uploaded beforehand.
type MediaInfo struct {
	// FileInfo comes from a file upload response.
	FileInfo string `json:"file_info,omitempty"`
}

// MessageReference makes a message a quote of another one.
type MessageReference struct {
	// MessageID is the quoted message id, from the ext data of an incoming
	// message or from the ref_idx of a sent message.
	MessageID string `json:"message_id,omitempty"`
}

// InputNotify reports that the bot is composing a reply.
type InputNotify struct {
	// InputType is documented as 1.
	InputType int `json:"input_type,omitempty"`
	// InputSecond is how long the state lasts, at most 60 seconds.
	InputSecond int `json:"input_second,omitempty"`
}

// MessageExtInfo carries the platform's extension fields of a sent message.
type MessageExtInfo struct {
	// RefIdx is the quote index, to be used as MessageReference.MessageID.
	RefIdx string `json:"ref_idx,omitempty"`
}

// MessageResponse is the result of a successful single-chat or group send.
type MessageResponse struct {
	// ID is the message id, usable for a later recall.
	ID string `json:"id"`
	// Timestamp is the send time in RFC3339, UTC+8.
	Timestamp string `json:"timestamp,omitempty"`
	// ExtInfo carries extra platform fields.
	ExtInfo *MessageExtInfo `json:"ext_info,omitempty"`
}

// StreamMessage is the request body of the streaming single-chat endpoint.
type StreamMessage struct {
	// InputMode is StreamInputAppend or StreamInputReplace.
	InputMode string `json:"input_mode,omitempty"`
	// InputState is StreamInputGenerating or StreamInputFinished.
	InputState int `json:"input_state,omitempty"`
	// Index counts the chunks from 0.
	Index int `json:"index,omitempty"`
	// ContentType is StreamContentText or StreamContentMarkdown.
	ContentType string `json:"content_type,omitempty"`
	// ContentRaw is the chunk text.
	ContentRaw string `json:"content_raw,omitempty"`
	// EventID replies to an event. Mutually exclusive with MsgID.
	EventID string `json:"event_id,omitempty"`
	// MsgID replies to a message. Mutually exclusive with EventID.
	MsgID string `json:"msg_id,omitempty"`
	// StreamMsgID continues a stream. The first chunk omits it and the
	// response id becomes its value for later chunks.
	StreamMsgID string `json:"stream_msg_id,omitempty"`
	// MsgSeq deduplicates replies.
	MsgSeq int `json:"msg_seq,omitempty"`
	// IsWakeup marks a recall message, skipping the reply-window check.
	IsWakeup bool `json:"is_wakeup,omitempty"`
}

// StreamMessageResponse is the result of one streaming chunk.
type StreamMessageResponse struct {
	MessageResponse
	// RemainMsgLen is how many characters of the stream may still be sent.
	RemainMsgLen int `json:"remain_msg_len,omitempty"`
}

// StreamID returns the id a later chunk must carry as StreamMessage.StreamMsgID.
//
// The platform generates it with the first chunk and returns it as the ordinary
// response id, which the documentation describes as "the id returned by the
// previous chunk", so it is the same field as MessageResponse.ID.
func (r *StreamMessageResponse) StreamID() string {
	if r == nil {
		return ""
	}
	return r.ID
}

// ChannelMessage is the request body of the channel and direct-message send
// endpoints.
//
// At least one of Content, Embed, Ark, Image and Markdown must be set.
type ChannelMessage struct {
	// Content is the plain text, supporting the documented inline format.
	Content string `json:"content,omitempty"`
	// Embed sends an embed card.
	Embed *MessageEmbed `json:"embed,omitempty"`
	// Ark sends a structured card.
	Ark *MessageArk `json:"ark,omitempty"`
	// MessageReference makes this message a quote.
	MessageReference *MessageReference `json:"message_reference,omitempty"`
	// Image is an image URL the platform transfers and sends.
	Image string `json:"image,omitempty"`
	// MsgID replies to a message.
	MsgID string `json:"msg_id,omitempty"`
	// EventID replies to an event.
	EventID string `json:"event_id,omitempty"`
	// Markdown sends markdown.
	Markdown *MessageMarkdown `json:"markdown,omitempty"`
}

// MessageEmbed is an embed card.
type MessageEmbed struct {
	// Title is the card title.
	Title string `json:"title,omitempty"`
	// Prompt is the notification text.
	Prompt string `json:"prompt,omitempty"`
	// Description is the card body.
	Description string `json:"description,omitempty"`
	// Thumbnail is the optional small image.
	Thumbnail *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	// Fields are the text rows of the card.
	Fields []MessageEmbedField `json:"fields,omitempty"`
}

// MessageEmbedThumbnail is the thumbnail of an embed card.
type MessageEmbedThumbnail struct {
	URL string `json:"url,omitempty"`
}

// MessageEmbedField is one text row of an embed card.
type MessageEmbedField struct {
	Name string `json:"name,omitempty"`
}

// MessageArk is a structured card built from a template and its variables.
type MessageArk struct {
	// TemplateID is the approved template id, for example 23, 24 or 37.
	TemplateID int `json:"template_id,omitempty"`
	// KV fills the template variables.
	KV []MessageArkKV `json:"kv,omitempty"`
}

// MessageArkKV fills one template variable.
//
// A scalar variable uses Value; an array variable uses Obj.
type MessageArkKV struct {
	Key   string          `json:"key,omitempty"`
	Value string          `json:"value,omitempty"`
	Obj   []MessageArkObj `json:"obj,omitempty"`
}

// MessageArkObj is one element of an array template variable.
type MessageArkObj struct {
	ObjKV []MessageArkObjKV `json:"obj_kv,omitempty"`
}

// MessageArkObjKV is one field of an array element.
type MessageArkObjKV struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// User is a user object.
//
// The channel message author, the group and single chat message author, and the
// mention list all use this shape. Which identifier is populated depends on the
// scenario: UserOpenID in a single chat, MemberOpenID in a group, and ID in a
// channel. The optional cross-application fields are only returned once the bot
// has applied for them.
type User struct {
	// ID is the user identifier, in OpenID form for the group and single chat
	// scenarios.
	ID string `json:"id,omitempty"`
	// Username is the nickname.
	Username string `json:"username,omitempty"`
	// Avatar is the avatar URL.
	Avatar string `json:"avatar,omitempty"`
	// Bot reports whether the account is a bot.
	Bot bool `json:"bot,omitempty"`
	// UnionOpenID is the cross-application user OpenID, and may be empty.
	UnionOpenID string `json:"union_openid,omitempty"`
	// UnionUserAccount is the cross-application user account, and may be empty.
	UnionUserAccount string `json:"union_user_account,omitempty"`
	// UserOpenID is the user OpenID, used in a single chat.
	UserOpenID string `json:"user_openid,omitempty"`
	// MemberOpenID is the group member OpenID, used in a group chat.
	MemberOpenID string `json:"member_openid,omitempty"`
	// MemberRole is the role in the group: member, admin or owner.
	MemberRole string `json:"member_role,omitempty"`
}

// ChannelMessageResponse is the channel message object returned after a send.
type ChannelMessageResponse struct {
	ID              string          `json:"id"`
	ChannelID       string          `json:"channel_id,omitempty"`
	GuildID         string          `json:"guild_id,omitempty"`
	Content         string          `json:"content,omitempty"`
	Timestamp       string          `json:"timestamp,omitempty"`
	TTS             bool            `json:"tts,omitempty"`
	MentionEveryone bool            `json:"mention_everyone,omitempty"`
	Author          *User           `json:"author,omitempty"`
	Embeds          []MessageEmbed  `json:"embeds,omitempty"`
	Pinned          bool            `json:"pinned,omitempty"`
	Type            int             `json:"type,omitempty"`
	Flags           int             `json:"flags,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

// DMS is a direct message session created with a guild member.
type DMS struct {
	// GuildID is the guild id of the private message session.
	GuildID string `json:"guild_id"`
	// ChannelID is the channel id of the private message session.
	ChannelID string `json:"channel_id"`
	// CreateTime is the session creation timestamp.
	CreateTime string `json:"create_time"`
}

// Validate checks a message against the documented rules whose breach the
// platform does not report.
//
// Only the silent failures are checked. A keyboard sent with a plain text
// message is accepted and its buttons are dropped, and a button missing a
// required field renders as nothing, so both are refused here instead of
// costing a round trip that only looks like it worked.
func (m *Message) Validate() error {
	if m == nil {
		return errors.New("qqbotsdk: message is nil")
	}
	if m.Keyboard == nil {
		return nil
	}
	if m.Markdown == nil || m.Markdown.Content == "" {
		return errors.New("qqbotsdk: a keyboard attaches to a markdown message, so set " +
			"MsgType to MsgTypeMarkdown and fill Markdown.Content; the platform " +
			"otherwise drops the buttons without reporting anything")
	}
	return m.Keyboard.validate()
}

// validate checks a keyboard's buttons, skipping a template keyboard, which
// carries an id instead of a layout.
func (k *Keyboard) validate() error {
	if k == nil || k.Content == nil {
		return nil
	}
	for i, row := range k.Content.Rows {
		for j := range row.Buttons {
			if err := row.Buttons[j].validate(); err != nil {
				return fmt.Errorf("qqbotsdk: keyboard row %d button %d: %w", i, j, err)
			}
		}
	}
	return nil
}

// validate checks the fields the documentation marks required.
//
// A field whose zero value is meaningful, such as render_data.style or
// action.type, cannot be told here from one that was never set, so those are
// left to the platform.
func (b *Button) validate() error {
	if b.RenderData == nil {
		return errors.New("render_data is required")
	}
	if b.RenderData.Label == "" {
		return errors.New("render_data.label is required")
	}
	if b.RenderData.VisitedLabel == "" {
		return errors.New("render_data.visited_label is required")
	}
	if b.Action == nil {
		return errors.New("action is required")
	}
	if b.Action.Permission == nil {
		return errors.New("action.permission is required")
	}
	if b.Action.Data == "" {
		return errors.New("action.data is required")
	}
	if b.Action.UnsupportTips == "" {
		return errors.New("action.unsupport_tips is required")
	}
	return nil
}

// SendC2CMessage sends a message to one user.
//
// The documented passive reply window is 60 minutes and at most 4 replies per
// message; an active message is subject to the platform's frequency limits.
func (c *Client) SendC2CMessage(ctx context.Context, userOpenID string, msg *Message) (*MessageResponse, error) {
	if userOpenID == "" {
		return nil, errors.New("qqbotsdk: SendC2CMessage needs a user openid")
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	var out MessageResponse
	path := "/v2/users/" + url.PathEscape(userOpenID) + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, path, msg, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendGroupMessage sends a message to one group.
//
// The documented passive reply window is 5 minutes and at most 5 replies per
// message. Group messages do not support streaming.
func (c *Client) SendGroupMessage(ctx context.Context, groupOpenID string, msg *Message) (*MessageResponse, error) {
	if groupOpenID == "" {
		return nil, errors.New("qqbotsdk: SendGroupMessage needs a group openid")
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	var out MessageResponse
	path := "/v2/groups/" + url.PathEscape(groupOpenID) + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, path, msg, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendC2CStreamMessage sends one chunk of a streaming single-chat reply.
//
// The first chunk omits StreamMsgID and its response id becomes the
// StreamMsgID of the following chunks, which increment Index. The last chunk
// sets InputState to StreamInputFinished.
func (c *Client) SendC2CStreamMessage(ctx context.Context, userOpenID string, msg *StreamMessage) (*StreamMessageResponse, error) {
	if userOpenID == "" {
		return nil, errors.New("qqbotsdk: SendC2CStreamMessage needs a user openid")
	}
	var out StreamMessageResponse
	path := "/v2/users/" + url.PathEscape(userOpenID) + "/stream_messages"
	if err := c.doJSON(ctx, http.MethodPost, path, msg, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendChannelMessage sends a message to a channel.
//
// The bot must be connected to the gateway while sending. At least one content
// field of msg must be set.
func (c *Client) SendChannelMessage(ctx context.Context, channelID string, msg *ChannelMessage) (*ChannelMessageResponse, error) {
	if channelID == "" {
		return nil, errors.New("qqbotsdk: SendChannelMessage needs a channel id")
	}
	var out ChannelMessageResponse
	path := "/channels/" + url.PathEscape(channelID) + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, path, msg, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendDirectMessage sends a private message in a direct message session.
//
// The parameters and the response match SendChannelMessage.
func (c *Client) SendDirectMessage(ctx context.Context, guildID string, msg *ChannelMessage) (*ChannelMessageResponse, error) {
	if guildID == "" {
		return nil, errors.New("qqbotsdk: SendDirectMessage needs a guild id")
	}
	var out ChannelMessageResponse
	path := "/dms/" + url.PathEscape(guildID) + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, path, msg, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateDirectMessageSession opens a private message session with a guild
// member. The bot and the user must share a guild.
func (c *Client) CreateDirectMessageSession(ctx context.Context, recipientID, sourceGuildID string) (*DMS, error) {
	if recipientID == "" || sourceGuildID == "" {
		return nil, errors.New("qqbotsdk: CreateDirectMessageSession needs a recipient and a source guild")
	}
	payload := struct {
		RecipientID   string `json:"recipient_id"`
		SourceGuildID string `json:"source_guild_id"`
	}{RecipientID: recipientID, SourceGuildID: sourceGuildID}

	var out DMS
	if err := c.doJSON(ctx, http.MethodPost, "/users/@me/dms", payload, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChannelMessageFile is the image attached to a multipart channel message.
type ChannelMessageFile struct {
	// FileName is the name reported to the platform.
	FileName string
	// Content is the image data.
	Content io.Reader
}

// SendChannelMessageMultipart sends a channel message as multipart/form-data,
// uploading an image in the same request.
//
// The platform requires object and array fields to be JSON encoded strings in
// a multipart body, which this method does. Pass a nil file to send a
// multipart request without an image attachment.
func (c *Client) SendChannelMessageMultipart(ctx context.Context, channelID string, msg *ChannelMessage, file *ChannelMessageFile) (*ChannelMessageResponse, error) {
	if channelID == "" {
		return nil, errors.New("qqbotsdk: SendChannelMessageMultipart needs a channel id")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Object fields are serialised as JSON strings, as the documentation
	// requires for multipart requests.
	if msg.Content != "" {
		if err := writer.WriteField("content", msg.Content); err != nil {
			return nil, fmt.Errorf("qqbotsdk: encode multipart content: %w", err)
		}
	}
	for field, value := range map[string]any{
		"embed":             msg.Embed,
		"ark":               msg.Ark,
		"markdown":          msg.Markdown,
		"message_reference": msg.MessageReference,
	} {
		if value == nil {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("qqbotsdk: encode multipart %s: %w", field, err)
		}
		if err := writer.WriteField(field, string(encoded)); err != nil {
			return nil, fmt.Errorf("qqbotsdk: write multipart %s: %w", field, err)
		}
	}
	for field, value := range map[string]string{
		"image":    msg.Image,
		"msg_id":   msg.MsgID,
		"event_id": msg.EventID,
	} {
		if value == "" {
			continue
		}
		if err := writer.WriteField(field, value); err != nil {
			return nil, fmt.Errorf("qqbotsdk: write multipart %s: %w", field, err)
		}
	}

	if file != nil && file.Content != nil {
		name := file.FileName
		if name == "" {
			name = "image"
		}
		part, err := writer.CreateFormFile("file_image", name)
		if err != nil {
			return nil, fmt.Errorf("qqbotsdk: create multipart file: %w", err)
		}
		if _, err := io.Copy(part, file.Content); err != nil {
			return nil, fmt.Errorf("qqbotsdk: write multipart file: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("qqbotsdk: close multipart body: %w", err)
	}

	var out ChannelMessageResponse
	path := "/channels/" + url.PathEscape(channelID) + "/messages"
	if err := c.do(ctx, http.MethodPost, path, &body, writer.FormDataContentType(), &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecallC2CMessage recalls a message the bot sent to one user.
//
// A message older than 2 minutes cannot be recalled.
func (c *Client) RecallC2CMessage(ctx context.Context, userOpenID, messageID string) error {
	if userOpenID == "" || messageID == "" {
		return errors.New("qqbotsdk: RecallC2CMessage needs a user openid and a message id")
	}
	path := "/v2/users/" + url.PathEscape(userOpenID) + "/messages/" + url.PathEscape(messageID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, openAPICall)
}

// RecallGroupMessage recalls a message in one group.
//
// A group administrator may also recall messages of ordinary members.
func (c *Client) RecallGroupMessage(ctx context.Context, groupOpenID, messageID string) error {
	if groupOpenID == "" || messageID == "" {
		return errors.New("qqbotsdk: RecallGroupMessage needs a group openid and a message id")
	}
	path := "/v2/groups/" + url.PathEscape(groupOpenID) + "/messages/" + url.PathEscape(messageID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, openAPICall)
}

// RecallChannelMessage recalls a message in a channel.
//
// hideTip hides the notice left in place of the recalled message.
func (c *Client) RecallChannelMessage(ctx context.Context, channelID, messageID string, hideTip bool) error {
	if channelID == "" || messageID == "" {
		return errors.New("qqbotsdk: RecallChannelMessage needs a channel id and a message id")
	}
	path := "/channels/" + url.PathEscape(channelID) + "/messages/" + url.PathEscape(messageID) +
		"?hidetip=" + strconv.FormatBool(hideTip)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, openAPICall)
}

// RecallDirectMessage recalls a private message in a direct message session.
//
// Only messages the bot sent may be recalled.
func (c *Client) RecallDirectMessage(ctx context.Context, guildID, messageID string, hideTip bool) error {
	if guildID == "" || messageID == "" {
		return errors.New("qqbotsdk: RecallDirectMessage needs a guild id and a message id")
	}
	path := "/dms/" + url.PathEscape(guildID) + "/messages/" + url.PathEscape(messageID) +
		"?hidetip=" + strconv.FormatBool(hideTip)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, openAPICall)
}
