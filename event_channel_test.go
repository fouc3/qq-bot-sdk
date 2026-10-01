package qqbotsdk

import (
	"testing"
)

// TestDecodeAtMessageCreate reproduces the documented channel message body.
func TestDecodeAtMessageCreate(t *testing.T) {
	payload := dispatchPayload(t, EventAtMessageCreate, `{
		"author": {"avatar": "http://thirdqq.qlogo.cn/0", "bot": false, "id": "1234", "username": "abc"},
		"channel_id": "100010",
		"content": "ndnnd",
		"guild_id": "18700000000001",
		"id": "0812345677890abcdef",
		"member": {"joined_at": "2021-04-12T16:34:42+08:00", "roles": ["1"]},
		"timestamp": "2021-05-20T15:14:58+08:00",
		"seq": 101
	}`)

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	message, ok := value.(*GuildMessage)
	if !ok {
		t.Fatalf("value = %T, want *GuildMessage", value)
	}

	if message.ID != "0812345677890abcdef" || message.Content != "ndnnd" {
		t.Errorf("message = %+v", message)
	}
	if message.ChannelID != "100010" || message.GuildID != "18700000000001" {
		t.Errorf("message = %+v", message)
	}
	if message.Author == nil || message.Author.ID != "1234" || message.Author.Avatar == "" {
		t.Errorf("Author = %+v", message.Author)
	}
	if message.Seq != 101 {
		t.Errorf("Seq = %d, want 101", message.Seq)
	}
	if message.Member == nil {
		t.Fatal("Member is nil")
	}
	if message.Member.JoinedAt != "2021-04-12T16:34:42+08:00" {
		t.Errorf("Member.JoinedAt = %q", message.Member.JoinedAt)
	}
	if len(message.Member.Roles) != 1 || message.Member.Roles[0] != "1" {
		t.Errorf("Member.Roles = %v", message.Member.Roles)
	}
}

// TestChannelMessageEventsShareTheBody checks that the three message events all
// decode into the same structure.
func TestChannelMessageEventsShareTheBody(t *testing.T) {
	body := `{"id":"M1","channel_id":"C1","content":"hi","seq_in_channel":"5"}`

	for _, eventType := range []string{EventAtMessageCreate, EventMessageCreate, EventDirectMessageCreate} {
		t.Run(eventType, func(t *testing.T) {
			message := mustDecode(t, dispatchPayload(t, eventType, body)).(*GuildMessage)
			if message.SeqInChannel != "5" {
				t.Errorf("SeqInChannel = %q, want the documented ordering field", message.SeqInChannel)
			}
		})
	}
}

// TestDecodeMessageDelete covers the wrapper carrying the deleted message.
func TestDecodeMessageDelete(t *testing.T) {
	body := `{
		"message": {"id": "M1", "channel_id": "C1", "content": "bye"},
		"op_user": {"id": "U1", "username": "abc"}
	}`

	for _, eventType := range []string{EventMessageDelete, EventPublicMessageDelete, EventDirectMessageDelete} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*MessageDelete)
			if data.Message == nil || data.Message.Content != "bye" {
				t.Errorf("Message = %+v", data.Message)
			}
			if data.OpUser == nil || data.OpUser.ID != "U1" {
				t.Errorf("OpUser = %+v", data.OpUser)
			}
		})
	}
}

// TestDecodeMessageAudited reproduces the documented audit body.
func TestDecodeMessageAudited(t *testing.T) {
	body := `{
		"audit_id": "5f60b782-d134-4628-93b8-9baa4b182f48",
		"audit_time": "2022-01-04T18:05:42+08:00",
		"channel_id": "1699792",
		"create_time": "2022-01-04T18:05:42+08:00",
		"guild_id": "46646271634786417",
		"message_id": "10d0df671a1231343431"
	}`

	for _, eventType := range []string{EventMessageAuditPass, EventMessageAuditReject} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*MessageAudited)
			if data.AuditID != "5f60b782-d134-4628-93b8-9baa4b182f48" {
				t.Errorf("AuditID = %q", data.AuditID)
			}
			if data.MessageID != "10d0df671a1231343431" {
				t.Errorf("MessageID = %q", data.MessageID)
			}
			if data.ChannelID != "1699792" || data.GuildID != "46646271634786417" {
				t.Errorf("data = %+v", data)
			}
		})
	}
}

// TestDecodeMessageReaction reproduces the documented reaction body.
func TestDecodeMessageReaction(t *testing.T) {
	body := `{
		"user_id": "1111222233333",
		"emoji": {"id": "277", "type": 1},
		"channel_id": "12345",
		"guild_id": "11110011112222",
		"target": {"id": "2", "type": 0}
	}`

	for _, eventType := range []string{EventMessageReactionAdd, EventMessageReactionRemove} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*MessageReaction)
			if data.UserID != "1111222233333" {
				t.Errorf("UserID = %q", data.UserID)
			}
			if data.Emoji == nil || data.Emoji.ID != "277" {
				t.Fatalf("Emoji = %+v", data.Emoji)
			}
			if !data.Emoji.IsBuiltinEmoji() {
				t.Error("a system emoji must report as built in")
			}
			if data.Target == nil || data.Target.ID != "2" || data.Target.Type != ReactionTargetMessage {
				t.Errorf("Target = %+v", data.Target)
			}
		})
	}
}

// TestEmojiIsBuiltinEdgeCases covers the documented type values.
func TestEmojiIsBuiltinEdgeCases(t *testing.T) {
	unicodeEmoji := &Emoji{ID: "🌹", Type: EmojiTypeEmoji}
	if unicodeEmoji.IsBuiltinEmoji() {
		t.Error("a Unicode emoji is not a built-in emoji")
	}
	if (*Emoji)(nil).IsBuiltinEmoji() {
		t.Error("a nil emoji must not report as built in")
	}
}

// TestReactionTargetConstants pins the documented target types.
func TestReactionTargetConstants(t *testing.T) {
	cases := map[int]string{
		ReactionTargetMessage: "message",
		ReactionTargetPost:    "post",
		ReactionTargetComment: "comment",
		ReactionTargetReply:   "reply",
	}
	if len(cases) != 4 {
		t.Fatalf("cases = %d, want the four documented target types", len(cases))
	}
	for value, name := range cases {
		if value < 0 || value > 3 {
			t.Errorf("%s = %d, want a value within 0..3", name, value)
		}
	}
}
