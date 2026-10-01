package qqbotsdk

import (
	"encoding/json"
	"testing"
)

// TestPayloadDecodesDocumentedShape decodes the exact JSON shape the payload
// documentation prints.
func TestPayloadDecodesDocumentedShape(t *testing.T) {
	const raw = `{
	  "id":"event_id",
	  "op": 0,
	  "d": {},
	  "s": 42,
	  "t": "GATEWAY_EVENT_NAME"
	}`

	var payload Payload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.ID != "event_id" {
		t.Errorf("ID = %q, want event_id", payload.ID)
	}
	if payload.Op != OpDispatch {
		t.Errorf("Op = %d, want %d", payload.Op, OpDispatch)
	}
	if payload.Type != "GATEWAY_EVENT_NAME" {
		t.Errorf("Type = %q, want GATEWAY_EVENT_NAME", payload.Type)
	}
	seq, ok := payload.Sequence()
	if !ok || seq != 42 {
		t.Errorf("Sequence() = %d, %v, want 42, true", seq, ok)
	}
}

// TestPayloadWithoutSequence checks that an absent s is distinguishable from 0.
func TestPayloadWithoutSequence(t *testing.T) {
	var payload Payload
	if err := json.Unmarshal([]byte(`{"op":11}`), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload.Sequence(); ok {
		t.Error("an absent s must report ok=false, so a zero sequence stays distinguishable")
	}
}

func TestPayloadDecodeData(t *testing.T) {
	payload := &Payload{
		Op:   OpDispatch,
		Type: EventC2CMessageCreate,
		Data: json.RawMessage(`{"id":"msg-1","content":"hello"}`),
	}

	var data struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := payload.DecodeData(&data); err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	if data.ID != "msg-1" || data.Content != "hello" {
		t.Errorf("data = %+v, want the decoded body", data)
	}
}

func TestPayloadDecodeDataErrors(t *testing.T) {
	empty := &Payload{Op: OpDispatch}
	if err := empty.DecodeData(&struct{}{}); err == nil {
		t.Error("decoding an absent body must fail")
	}

	bad := &Payload{Data: json.RawMessage(`{"id":123}`)}
	var target struct {
		ID string `json:"id"`
	}
	if err := bad.DecodeData(&target); err == nil {
		t.Error("decoding a mismatched body must fail")
	}
}

func TestOpCodeString(t *testing.T) {
	cases := map[OpCode]string{
		OpDispatch:        "Dispatch",
		OpHeartbeat:       "Heartbeat",
		OpIdentify:        "Identify",
		OpResume:          "Resume",
		OpReconnect:       "Reconnect",
		OpInvalidSession:  "Invalid Session",
		OpHello:           "Hello",
		OpHeartbeatACK:    "Heartbeat ACK",
		OpHTTPCallbackACK: "HTTP Callback ACK",
		OpCallbackVerify:  "回调地址验证",
		OpCode(99):        "OpCode(99)",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestIntentBitsMatchDocumentation pins the bit assigned to each category, since
// a wrong bit silently subscribes to the wrong events.
func TestIntentBitsMatchDocumentation(t *testing.T) {
	cases := map[string]struct {
		intent Intent
		shift  uint
	}{
		"GUILDS":                  {IntentGuilds, 0},
		"GUILD_MEMBERS":           {IntentGuildMembers, 1},
		"GUILD_MESSAGES":          {IntentGuildMessages, 9},
		"GUILD_MESSAGE_REACTIONS": {IntentGuildMessageReactions, 10},
		"DIRECT_MESSAGE":          {IntentDirectMessage, 12},
		"GROUP_AND_C2C_EVENT":     {IntentGroupAndC2CEvent, 25},
		"INTERACTION":             {IntentInteraction, 26},
		"MESSAGE_AUDIT":           {IntentMessageAudit, 27},
		"FORUMS_EVENT":            {IntentForumsEvent, 28},
		"AUDIO_ACTION":            {IntentAudioAction, 29},
		"PUBLIC_GUILD_MESSAGES":   {IntentPublicGuildMessages, 30},
	}
	for name, tc := range cases {
		if want := Intent(1) << tc.shift; tc.intent != want {
			t.Errorf("%s = %d, want 1<<%d = %d", name, tc.intent, tc.shift, want)
		}
	}
}

// TestDocumentedIntentExample reproduces the arithmetic in the documentation:
// subscribing to PUBLIC_GUILD_MESSAGES and GUILD_MEMBERS is 1<<30 | 1<<1.
func TestDocumentedIntentExample(t *testing.T) {
	got := IntentsFor(IntentPublicGuildMessages, IntentGuildMembers)
	want := Intent(0) | 1<<30 | 1<<1

	if got != want {
		t.Errorf("IntentsFor(...) = %d, want %d", got, want)
	}
	if !got.Has(IntentPublicGuildMessages) || !got.Has(IntentGuildMembers) {
		t.Error("the combined intent must report both categories")
	}
	if got.Has(IntentGuilds) {
		t.Error("the combined intent must not include an unrequested category")
	}
}

func TestIntentForEvent(t *testing.T) {
	cases := map[string]Intent{
		EventGuildCreate:          IntentGuilds,
		EventChannelDelete:        IntentGuilds,
		EventGuildMemberAdd:       IntentGuildMembers,
		EventMessageCreate:        IntentGuildMessages,
		EventMessageReactionAdd:   IntentGuildMessageReactions,
		EventDirectMessageCreate:  IntentDirectMessage,
		EventGroupAtMessageCreate: IntentGroupAndC2CEvent,
		EventC2CMessageCreate:     IntentGroupAndC2CEvent,
		EventInteractionCreate:    IntentInteraction,
		EventMessageAuditPass:     IntentMessageAudit,
		EventForumThreadCreate:    IntentForumsEvent,
		EventAudioStart:           IntentAudioAction,
		EventAtMessageCreate:      IntentPublicGuildMessages,
		EventPublicMessageDelete:  IntentPublicGuildMessages,
	}
	for eventType, want := range cases {
		if got := IntentForEvent(eventType); got != want {
			t.Errorf("IntentForEvent(%q) = %d, want %d", eventType, got, want)
		}
	}
	if got := IntentForEvent("NOT_A_REAL_EVENT"); got != 0 {
		t.Errorf("IntentForEvent(unknown) = %d, want 0", got)
	}
}

// TestEveryEventTypeHasAnIntent guards against a documented event being added
// without its subscribing category.
func TestEveryEventTypeHasAnIntent(t *testing.T) {
	eventTypes := []string{
		EventGuildCreate, EventGuildUpdate, EventGuildDelete,
		EventChannelCreate, EventChannelUpdate, EventChannelDelete,
		EventGuildMemberAdd, EventGuildMemberUpdate, EventGuildMemberRemove,
		EventMessageCreate, EventMessageDelete,
		EventMessageReactionAdd, EventMessageReactionRemove,
		EventDirectMessageCreate, EventDirectMessageDelete,
		EventC2CMessageCreate, EventFriendAdd, EventFriendDel,
		EventC2CMsgReject, EventC2CMsgReceive, EventGroupAtMessageCreate,
		EventGroupAddRobot, EventGroupDelRobot, EventGroupMsgReject,
		EventGroupMsgReceive, EventInteractionCreate,
		EventMessageAuditPass, EventMessageAuditReject,
		EventForumThreadCreate, EventForumThreadUpdate, EventForumThreadDelete,
		EventForumPostCreate, EventForumPostDelete, EventForumReplyCreate,
		EventForumReplyDelete, EventForumPublishAuditResult,
		EventAudioStart, EventAudioFinish, EventAudioOnMic, EventAudioOffMic,
		EventAtMessageCreate, EventPublicMessageDelete,
	}
	for _, eventType := range eventTypes {
		if IntentForEvent(eventType) == 0 {
			t.Errorf("event %q has no subscribing intent", eventType)
		}
	}
}

func TestNewEventCarriesDeliveryContext(t *testing.T) {
	payload := &Payload{Op: OpDispatch, Type: EventAtMessageCreate}
	event := NewEvent(payload, TransportNameWebSocket)

	if event.Payload != payload {
		t.Error("the event must expose the payload it wraps")
	}
	if event.Transport != TransportNameWebSocket {
		t.Errorf("Transport = %q, want %q", event.Transport, TransportNameWebSocket)
	}
	if event.ReceivedAt.IsZero() {
		t.Error("ReceivedAt must be stamped")
	}
	// The documented fields stay reachable through the embedded payload.
	if event.Type != EventAtMessageCreate || event.Op != OpDispatch {
		t.Error("the embedded payload fields must stay accessible")
	}
}
