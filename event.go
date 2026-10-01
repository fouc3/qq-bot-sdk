package qqbotsdk

import (
	"encoding/json"
	"fmt"
)

// OpCode is the opcode of a gateway payload.
//
// The same payload structure is used by both webhook and websocket delivery.
type OpCode int

// Opcodes from the official payload documentation.
const (
	// OpDispatch is a server push of an event.
	OpDispatch OpCode = 0
	// OpHeartbeat is sent by the client, and may be sent by the server.
	OpHeartbeat OpCode = 1
	// OpIdentify authenticates a websocket connection.
	OpIdentify OpCode = 2
	// OpResume restores a dropped websocket session.
	OpResume OpCode = 6
	// OpReconnect asks the client to reconnect.
	OpReconnect OpCode = 7
	// OpInvalidSession reports a bad identify or resume.
	OpInvalidSession OpCode = 9
	// OpHello is the first message after a websocket connects.
	OpHello OpCode = 10
	// OpHeartbeatACK acknowledges a heartbeat.
	OpHeartbeatACK OpCode = 11
	// OpHTTPCallbackACK is the webhook reply acknowledging a push.
	OpHTTPCallbackACK OpCode = 12
	// OpCallbackVerify is the platform validating a webhook callback address.
	OpCallbackVerify OpCode = 13
)

// String returns the documented name of the opcode, or its number.
func (o OpCode) String() string {
	switch o {
	case OpDispatch:
		return "Dispatch"
	case OpHeartbeat:
		return "Heartbeat"
	case OpIdentify:
		return "Identify"
	case OpResume:
		return "Resume"
	case OpReconnect:
		return "Reconnect"
	case OpInvalidSession:
		return "Invalid Session"
	case OpHello:
		return "Hello"
	case OpHeartbeatACK:
		return "Heartbeat ACK"
	case OpHTTPCallbackACK:
		return "HTTP Callback ACK"
	case OpCallbackVerify:
		return "回调地址验证"
	}
	return fmt.Sprintf("OpCode(%d)", int(o))
}

// Payload is the transport structure shared by webhook and websocket, exactly
// as documented:
//
//	{
//	  "id": "event_id",
//	  "op": 0,
//	  "d": {},
//	  "s": 42,
//	  "t": "GATEWAY_EVENT_NAME"
//	}
//
// The documented field descriptions are:
//
//   - id: event id
//   - op: opcode, see OpCode
//   - s:  sequence number, unique per downstream message; the client must echo
//     the latest one it received when sending a heartbeat
//   - t:  event type, meaningful when op is OpDispatch
//   - d:  event body; its shape depends on t and is only known when op is
//     OpDispatch
type Payload struct {
	// ID is the event id.
	ID string `json:"id,omitempty"`
	// Op is the opcode.
	Op OpCode `json:"op"`
	// Data is the raw event body. Decode it with DecodeData once Op is known.
	Data json.RawMessage `json:"d,omitempty"`
	// Seq is the sequence number of a downstream message.
	Seq *int64 `json:"s,omitempty"`
	// Type is the event type, set when Op is OpDispatch.
	Type string `json:"t,omitempty"`
}

// DecodeData unmarshals the event body into v.
//
// The body shape depends on the event type, so the caller decides what to
// decode it into:
//
//	var data qqbotsdk.C2CMessageCreateData
//	if err := payload.DecodeData(&data); err != nil { ... }
func (p *Payload) DecodeData(v any) error {
	if len(p.Data) == 0 {
		return fmt.Errorf("qqbotsdk: payload has no body to decode")
	}
	if err := json.Unmarshal(p.Data, v); err != nil {
		return fmt.Errorf("qqbotsdk: decode event body: %w", err)
	}
	return nil
}

// Sequence returns the sequence number and whether one was present.
func (p *Payload) Sequence() (int64, bool) {
	if p.Seq == nil {
		return 0, false
	}
	return *p.Seq, true
}

// Intent is a bitmask of the event categories a bot subscribes to.
//
// Categories are combined with bitwise OR, e.g. GUILDS | PUBLIC_GUILD_MESSAGES.
// The platform rejects an intent the bot is not authorized for by closing the
// websocket connection.
type Intent uint64

// Event subscription intents, one bit per category.
//
// GUILDS, GUILD_MEMBERS and PUBLIC_GUILD_MESSAGES are basic and available by
// default; the others must be applied for.
const (
	IntentGuilds                Intent = 1 << 0
	IntentGuildMembers          Intent = 1 << 1
	IntentGuildMessages         Intent = 1 << 9 // 私域 only
	IntentGuildMessageReactions Intent = 1 << 10
	IntentDirectMessage         Intent = 1 << 12
	IntentGroupAndC2CEvent      Intent = 1 << 25
	IntentInteraction           Intent = 1 << 26
	IntentMessageAudit          Intent = 1 << 27
	IntentForumsEvent           Intent = 1 << 28 // 私域 only
	IntentAudioAction           Intent = 1 << 29
	IntentPublicGuildMessages   Intent = 1 << 30
)

// IntentsFor returns the combined intent subscribing to every named category.
func IntentsFor(intents ...Intent) Intent {
	var combined Intent
	for _, intent := range intents {
		combined |= intent
	}
	return combined
}

// Has reports whether the intent mask includes the given category.
func (i Intent) Has(intent Intent) bool {
	return i&intent != 0
}

// Event type names carried by the t field of a dispatch payload.
//
// They are grouped under the intent that subscribes to them.
const (
	// IntentGuilds.
	EventGuildCreate   = "GUILD_CREATE"
	EventGuildUpdate   = "GUILD_UPDATE"
	EventGuildDelete   = "GUILD_DELETE"
	EventChannelCreate = "CHANNEL_CREATE"
	EventChannelUpdate = "CHANNEL_UPDATE"
	EventChannelDelete = "CHANNEL_DELETE"

	// IntentGuildMembers.
	EventGuildMemberAdd    = "GUILD_MEMBER_ADD"
	EventGuildMemberUpdate = "GUILD_MEMBER_UPDATE"
	EventGuildMemberRemove = "GUILD_MEMBER_REMOVE"

	// IntentGuildMessages (私域 only).
	EventMessageCreate = "MESSAGE_CREATE"
	EventMessageDelete = "MESSAGE_DELETE"

	// IntentGuildMessageReactions.
	EventMessageReactionAdd    = "MESSAGE_REACTION_ADD"
	EventMessageReactionRemove = "MESSAGE_REACTION_REMOVE"

	// IntentDirectMessage.
	EventDirectMessageCreate = "DIRECT_MESSAGE_CREATE"
	EventDirectMessageDelete = "DIRECT_MESSAGE_DELETE"

	// IntentGroupAndC2CEvent.
	EventC2CMessageCreate     = "C2C_MESSAGE_CREATE"
	EventFriendAdd            = "FRIEND_ADD"
	EventFriendDel            = "FRIEND_DEL"
	EventC2CMsgReject         = "C2C_MSG_REJECT"
	EventC2CMsgReceive        = "C2C_MSG_RECEIVE"
	EventGroupAtMessageCreate = "GROUP_AT_MESSAGE_CREATE"
	EventGroupAddRobot        = "GROUP_ADD_ROBOT"
	EventGroupDelRobot        = "GROUP_DEL_ROBOT"
	EventGroupMsgReject       = "GROUP_MSG_REJECT"
	EventGroupMsgReceive      = "GROUP_MSG_RECEIVE"

	// IntentInteraction.
	EventInteractionCreate = "INTERACTION_CREATE"

	// IntentMessageAudit.
	EventMessageAuditPass   = "MESSAGE_AUDIT_PASS"
	EventMessageAuditReject = "MESSAGE_AUDIT_REJECT"

	// IntentForumsEvent (私域 only).
	EventForumThreadCreate       = "FORUM_THREAD_CREATE"
	EventForumThreadUpdate       = "FORUM_THREAD_UPDATE"
	EventForumThreadDelete       = "FORUM_THREAD_DELETE"
	EventForumPostCreate         = "FORUM_POST_CREATE"
	EventForumPostDelete         = "FORUM_POST_DELETE"
	EventForumReplyCreate        = "FORUM_REPLY_CREATE"
	EventForumReplyDelete        = "FORUM_REPLY_DELETE"
	EventForumPublishAuditResult = "FORUM_PUBLISH_AUDIT_RESULT"

	// IntentAudioAction.
	EventAudioStart  = "AUDIO_START"
	EventAudioFinish = "AUDIO_FINISH"
	EventAudioOnMic  = "AUDIO_ON_MIC"
	EventAudioOffMic = "AUDIO_OFF_MIC"

	// IntentPublicGuildMessages.
	EventAtMessageCreate     = "AT_MESSAGE_CREATE"
	EventPublicMessageDelete = "PUBLIC_MESSAGE_DELETE"

	// Lifecycle events delivered on the websocket connection itself.
	EventReady   = "READY"
	EventResumed = "RESUMED"
)

// IntentForEvent returns the intent category that carries the given event type,
// or 0 when the type is unknown.
func IntentForEvent(eventType string) Intent {
	if intent, ok := eventIntents[eventType]; ok {
		return intent
	}
	return 0
}

// eventIntents maps every documented event type to its subscribing intent.
var eventIntents = map[string]Intent{
	EventGuildCreate: IntentGuilds, EventGuildUpdate: IntentGuilds,
	EventGuildDelete: IntentGuilds, EventChannelCreate: IntentGuilds,
	EventChannelUpdate: IntentGuilds, EventChannelDelete: IntentGuilds,

	EventGuildMemberAdd: IntentGuildMembers, EventGuildMemberUpdate: IntentGuildMembers,
	EventGuildMemberRemove: IntentGuildMembers,

	EventMessageCreate: IntentGuildMessages, EventMessageDelete: IntentGuildMessages,

	EventMessageReactionAdd:    IntentGuildMessageReactions,
	EventMessageReactionRemove: IntentGuildMessageReactions,

	EventDirectMessageCreate: IntentDirectMessage, EventDirectMessageDelete: IntentDirectMessage,

	EventC2CMessageCreate: IntentGroupAndC2CEvent, EventFriendAdd: IntentGroupAndC2CEvent,
	EventFriendDel: IntentGroupAndC2CEvent, EventC2CMsgReject: IntentGroupAndC2CEvent,
	EventC2CMsgReceive: IntentGroupAndC2CEvent, EventGroupAtMessageCreate: IntentGroupAndC2CEvent,
	EventGroupAddRobot: IntentGroupAndC2CEvent, EventGroupDelRobot: IntentGroupAndC2CEvent,
	EventGroupMsgReject: IntentGroupAndC2CEvent, EventGroupMsgReceive: IntentGroupAndC2CEvent,

	EventInteractionCreate: IntentInteraction,

	EventMessageAuditPass: IntentMessageAudit, EventMessageAuditReject: IntentMessageAudit,

	EventForumThreadCreate: IntentForumsEvent, EventForumThreadUpdate: IntentForumsEvent,
	EventForumThreadDelete: IntentForumsEvent, EventForumPostCreate: IntentForumsEvent,
	EventForumPostDelete: IntentForumsEvent, EventForumReplyCreate: IntentForumsEvent,
	EventForumReplyDelete: IntentForumsEvent, EventForumPublishAuditResult: IntentForumsEvent,

	EventAudioStart: IntentAudioAction, EventAudioFinish: IntentAudioAction,
	EventAudioOnMic: IntentAudioAction, EventAudioOffMic: IntentAudioAction,

	EventAtMessageCreate:     IntentPublicGuildMessages,
	EventPublicMessageDelete: IntentPublicGuildMessages,
}
