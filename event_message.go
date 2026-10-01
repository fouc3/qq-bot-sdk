package qqbotsdk

import (
	"strings"
)

// Message content types carried by an incoming message event.
//
// These are the types the platform sends, which differ from the MsgType
// constants used when sending.
const (
	// EventMsgTypeText is a plain text message.
	EventMsgTypeText = 0
	// EventMsgTypeArk is a structured card message, carried in ARKData.
	EventMsgTypeArk = 3
	// EventMsgTypeParallel is a parallel message.
	EventMsgTypeParallel = 101
	// EventMsgTypeChatRecord is a chat record.
	EventMsgTypeChatRecord = 102
	// EventMsgTypeQuote is a quote, whose quoted content is in MsgElements.
	EventMsgTypeQuote = 103
)

// ARK card type identifiers, as documented for ARKData.ArkType.
const (
	ARKTypeTuwen         = "tuwen"
	ARKTypeFeed          = "feed"
	ARKTypeMiniApp       = "miniapp"
	ARKTypeMap           = "map"
	ARKTypeContactCard   = "contact_card"
	ARKTypeVideoShare    = "video_share"
	ARKTypeMusicTogether = "music_together"
	ARKTypePicture       = "picture"
)

// Attachment content types, as documented for MessageAttachment.ContentType.
const (
	AttachmentContentTypeVoice = "voice"
	AttachmentContentTypeJPEG  = "image/jpeg"
	AttachmentContentTypePNG   = "image/png"
	AttachmentContentTypeGIF   = "image/gif"
	AttachmentContentTypeMP4   = "video/mp4"
	AttachmentContentTypeFile  = "file"
)

// MessageScene is the scene context of an incoming message.
type MessageScene struct {
	// Source is the scene origin; default is the normal chat window.
	Source string `json:"source,omitempty"`
	// Ext holds key=value pairs:
	//
	//	msg_idx      index of this message, used for de-duplication
	//	ref_msg_idx  index of the quoted message
	//	auth_token   an authorization token
	//
	// Read them with ExtValue rather than parsing the list yourself.
	Ext []string `json:"ext,omitempty"`
}

// ExtValue returns the value of a key in the extension list.
func (m *MessageScene) ExtValue(key string) (string, bool) {
	if m == nil {
		return "", false
	}
	prefix := key + "="
	for _, entry := range m.Ext {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):], true
		}
	}
	return "", false
}

// MsgIdx returns the documented msg_idx value, used to de-duplicate a message
// the platform may push more than once.
func (m *MessageScene) MsgIdx() (string, bool) { return m.ExtValue("msg_idx") }

// RefMsgIdx returns the documented ref_msg_idx value, the index of the message
// this one quotes.
func (m *MessageScene) RefMsgIdx() (string, bool) { return m.ExtValue("ref_msg_idx") }

// AuthToken returns the documented auth_token value.
func (m *MessageScene) AuthToken() (string, bool) { return m.ExtValue("auth_token") }

// MessageAttachment is a file, image, voice clip or video attached to a
// message.
type MessageAttachment struct {
	// URL downloads the attachment.
	URL string `json:"url,omitempty"`
	// Filename is the file name.
	Filename string `json:"filename,omitempty"`
	// Width is the pixel width, absent for a non-image attachment.
	Width int `json:"width,omitempty"`
	// Height is the pixel height, absent for a non-image attachment.
	Height int `json:"height,omitempty"`
	// Size is the size in bytes.
	Size int64 `json:"size,omitempty"`
	// ContentType is one of the AttachmentContentType constants.
	ContentType string `json:"content_type,omitempty"`
	// VoiceWavURL is the WAV conversion of a voice message.
	VoiceWavURL string `json:"voice_wav_url,omitempty"`
	// ASRReferText is the speech recognition result of a voice message.
	ASRReferText string `json:"asr_refer_text,omitempty"`
}

// ARKData is the structured card of an incoming message.
type ARKData struct {
	// Prompt is the user-facing hint of the card.
	Prompt string `json:"prompt,omitempty"`
	// ArkType is one of the ARKType constants.
	ArkType string `json:"ark_type,omitempty"`
	// ArkName is the Chinese name of the card type.
	ArkName string `json:"ark_name,omitempty"`
	// Fields holds the card fields, whose documented keys include tag, tags,
	// title, desc, jump_url, preview, source, source_logo, tag_icon, nickname,
	// avatar and address.
	Fields map[string]any `json:"fields,omitempty"`
}

// MsgElement is one entry of a message, used by a quote and by nested
// structures.
type MsgElement struct {
	// MsgIdx is the index of the referenced message.
	MsgIdx string `json:"msg_idx,omitempty"`
	// Author is the sender of this element.
	Author *User `json:"author,omitempty"`
	// MessageType is one of the EventMsgType constants.
	MessageType int `json:"message_type,omitempty"`
	// Content is the text of this element.
	Content string `json:"content,omitempty"`
	// Attachments are the attachments of this element.
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	// ArkData is set when MessageType is EventMsgTypeArk.
	ArkData *ARKData `json:"ark_data,omitempty"`
	// MsgElements are nested elements; the documentation describes the
	// structure as recursive.
	MsgElements []MsgElement `json:"msg_elements,omitempty"`
}

// C2CMessageCreateData is the body of C2C_MESSAGE_CREATE: a user sent the bot a
// single chat message.
//
// The same msg_id may be pushed more than once; de-duplicate with
// MessageScene.MsgIdx.
type C2CMessageCreateData struct {
	// ID is the message id, usable for a passive reply and for recall.
	ID string `json:"id"`
	// Author is the sender, with UserOpenID populated.
	Author *User `json:"author"`
	// Content is the text of the message.
	Content string `json:"content,omitempty"`
	// Timestamp is the send time in RFC3339.
	Timestamp string `json:"timestamp,omitempty"`
	// MessageType is one of the EventMsgType constants.
	MessageType int `json:"message_type,omitempty"`
	// MessageScene is the scene context.
	MessageScene *MessageScene `json:"message_scene,omitempty"`
	// Attachments are the attached images, files or voice clips.
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	// ArkData is set when MessageType is EventMsgTypeArk.
	ArkData *ARKData `json:"ark_data,omitempty"`
	// MsgElements carry the quoted content when MessageType is
	// EventMsgTypeQuote.
	MsgElements []MsgElement `json:"msg_elements,omitempty"`
}

// GroupMessageCreateData is the body of GROUP_AT_MESSAGE_CREATE and
// GROUP_MESSAGE_CREATE: a message was sent in a group.
//
// The two events carry the same structure; the first is triggered when the bot
// is mentioned, the second when the bot receives every group message. The
// content field has the mention prefix already removed.
type GroupMessageCreateData struct {
	// ID is the message id, usable for a passive reply and for recall.
	ID string `json:"id"`
	// Author is the sender, with MemberOpenID populated.
	Author *User `json:"author"`
	// Content is the text of the message.
	Content string `json:"content,omitempty"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid,omitempty"`
	// Timestamp is the send time in RFC3339.
	Timestamp string `json:"timestamp,omitempty"`
	// MessageType is one of the EventMsgType constants.
	MessageType int `json:"message_type,omitempty"`
	// MessageScene is the scene context.
	MessageScene *MessageScene `json:"message_scene,omitempty"`
	// Attachments are the attached images, files or voice clips.
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	// Mentions are the mentioned users, excluding the bot itself.
	Mentions []User `json:"mentions,omitempty"`
	// ArkData is set when MessageType is EventMsgTypeArk.
	ArkData *ARKData `json:"ark_data,omitempty"`
	// MsgElements carry the quoted content when MessageType is
	// EventMsgTypeQuote.
	MsgElements []MsgElement `json:"msg_elements,omitempty"`
}

// IsVoice reports whether the attachment is a voice message.
func (a MessageAttachment) IsVoice() bool {
	return a.ContentType == AttachmentContentTypeVoice
}

// IsImage reports whether the attachment is one of the documented image types.
func (a MessageAttachment) IsImage() bool {
	switch a.ContentType {
	case AttachmentContentTypeJPEG, AttachmentContentTypePNG, AttachmentContentTypeGIF:
		return true
	default:
		return false
	}
}
