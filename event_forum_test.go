package qqbotsdk

import (
	"encoding/json"
	"testing"
)

// TestDecodeForumThreadEvent reproduces the documented thread example, whose
// ids are numbers and whose title and content are rich text arrays.
func TestDecodeForumThreadEvent(t *testing.T) {
	payload := dispatchPayload(t, EventForumThreadCreate, `{
		"guild_id": 47129941624960822,
		"channel_id": 1661124,
		"author_id": 144115218182563108,
		"thread_info": {
			"thread_id": "B_7c02cb615f8904001441152181825631080X60",
			"title": [{"type": 1, "text_info": {"text": "Test"}}],
			"content": [
				{"type": 1, "text_info": {"text": "tencent "}},
				{"type": 5, "channel_info": {"channel_id": 1505272, "channel_name": "#隐私子频道"}},
				{"type": 3, "url_info": {"url": "https://apple.com", "display_text": "Apple"}}
			],
			"date_time": "2021-12-30T15:17:34+08:00"
		}
	}`)

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	event, ok := value.(*ForumThreadEvent)
	if !ok {
		t.Fatalf("value = %T, want *ForumThreadEvent", value)
	}

	// The numeric form in the example must become the same string the field
	// table documents.
	if event.GuildID != "47129941624960822" {
		t.Errorf("GuildID = %q, want the numeric id as a string", event.GuildID)
	}
	if event.ChannelID != "1661124" || event.AuthorID != "144115218182563108" {
		t.Errorf("event = %+v", event)
	}

	if event.ThreadInfo == nil {
		t.Fatal("ThreadInfo is nil")
	}
	if event.ThreadInfo.ThreadID != "B_7c02cb615f8904001441152181825631080X60" {
		t.Errorf("ThreadID = %q", event.ThreadInfo.ThreadID)
	}
	if got := event.ThreadInfo.Title.PlainText(); got != "Test" {
		t.Errorf("title = %q, want Test", got)
	}
	if got := event.ThreadInfo.Content.PlainText(); got != "tencent #隐私子频道Apple" {
		t.Errorf("content = %q", got)
	}
	if len(event.ThreadInfo.Content.Elements) != 3 {
		t.Fatalf("elements = %d, want 3", len(event.ThreadInfo.Content.Elements))
	}
	if event.ThreadInfo.Content.Elements[1].ChannelInfo.ChannelID != 1505272 {
		t.Errorf("channel element = %+v", event.ThreadInfo.Content.Elements[1])
	}
}

// TestDecodeForumPostEvent reproduces the documented post example, which
// carries emoji elements.
func TestDecodeForumPostEvent(t *testing.T) {
	payload := dispatchPayload(t, EventForumPostCreate, `{
		"guild_id": "47129941624960822",
		"channel_id": "1661124",
		"author_id": "144115218182563108",
		"post_info": {
			"thread_id": "B_6d02bb61e45b0d001441152181867088220X60",
			"post_id": "c_1500cb611f950a001441152181825631080X60",
			"content": [
				{"type": 1, "text_info": {"text": "test"}},
				{"type": 4, "emoji_info": {"id": 109, "type": "1"}},
				{"type": 1, "text_info": {"text": "111"}}
			],
			"date_time": "2021-12-30T15:17:34+08:00"
		}
	}`)

	event := mustDecode(t, payload).(*ForumPostEvent)
	if event.PostInfo == nil {
		t.Fatal("PostInfo is nil")
	}
	if event.PostInfo.PostID != "c_1500cb611f950a001441152181825631080X60" {
		t.Errorf("PostID = %q", event.PostInfo.PostID)
	}

	elements := event.PostInfo.Content.Elements
	if len(elements) != 3 {
		t.Fatalf("elements = %d, want 3", len(elements))
	}
	// The example writes the emoji id as a number and the type as a string,
	// against the field table; both must still decode.
	if elements[1].EmojiInfo == nil {
		t.Fatalf("element = %+v", elements[1])
	}
	if elements[1].EmojiInfo.ID == "" || elements[1].EmojiInfo.Type != "1" {
		t.Errorf("EmojiInfo = %+v", elements[1].EmojiInfo)
	}
}

// TestDecodeForumReplyEvent covers the reply body.
func TestDecodeForumReplyEvent(t *testing.T) {
	payload := dispatchPayload(t, EventForumReplyCreate, `{
		"guild_id": 47129941624960822,
		"channel_id": 1661124,
		"author_id": 144115218182563108,
		"reply_info": {
			"thread_id": "B_1", "post_id": "c_1", "reply_id": "r_1",
			"content": [{"type": 1, "text_info": {"text": "回复内容"}}],
			"date_time": "2021-12-30T15:17:34+08:00"
		}
	}`)

	event := mustDecode(t, payload).(*ForumReplyEvent)
	if event.GuildID != "47129941624960822" {
		t.Errorf("GuildID = %q", event.GuildID)
	}
	if event.ReplyInfo == nil || event.ReplyInfo.ReplyID != "r_1" {
		t.Fatalf("ReplyInfo = %+v", event.ReplyInfo)
	}
	if got := event.ReplyInfo.Content.PlainText(); got != "回复内容" {
		t.Errorf("content = %q", got)
	}
}

// TestDecodeForumAuditResult covers the audit event.
func TestDecodeForumAuditResult(t *testing.T) {
	payload := dispatchPayload(t, EventForumPublishAuditResult, `{
		"guild_id": "G1", "channel_id": "C1", "author_id": "A1",
		"thread_id": "T1", "post_id": "P1", "reply_id": "R1",
		"type": 2, "result": 1, "err_msg": "内容不合规"
	}`)

	event := mustDecode(t, payload).(*ForumAuditResult)
	if event.Type != ForumAuditPublishPost {
		t.Errorf("Type = %d, want the post audit type", event.Type)
	}
	if event.Result != ForumAuditFailure {
		t.Errorf("Result = %d, want a failure", event.Result)
	}
	if event.ErrMsg != "内容不合规" {
		t.Errorf("ErrMsg = %q", event.ErrMsg)
	}
	if event.ReplyID != "R1" {
		t.Errorf("ReplyID = %q", event.ReplyID)
	}
}

// TestRichTextValueAcceptsBothForms covers the disagreement between the field
// table, which says string, and every example, which shows an array.
func TestRichTextValueAcceptsBothForms(t *testing.T) {
	var fromString RichTextValue
	if err := json.Unmarshal([]byte(`"纯文本标题"`), &fromString); err != nil {
		t.Fatalf("string form: %v", err)
	}
	if fromString.PlainText() != "纯文本标题" {
		t.Errorf("PlainText = %q", fromString.PlainText())
	}
	if len(fromString.Elements) != 0 {
		t.Errorf("Elements = %v, want none for the string form", fromString.Elements)
	}

	var fromArray RichTextValue
	if err := json.Unmarshal([]byte(`[{"type":1,"text_info":{"text":"a"}}]`), &fromArray); err != nil {
		t.Fatalf("array form: %v", err)
	}
	if fromArray.PlainText() != "a" {
		t.Errorf("PlainText = %q", fromArray.PlainText())
	}

	var fromNull RichTextValue
	if err := json.Unmarshal([]byte(`null`), &fromNull); err != nil {
		t.Fatalf("null form: %v", err)
	}
	if fromNull.PlainText() != "" {
		t.Errorf("PlainText = %q, want empty", fromNull.PlainText())
	}

	var empty RichTextValue
	if err := json.Unmarshal([]byte(``), &empty); err == nil {
		t.Error("an empty body must be reported")
	}
}

// TestRichTextValueMarshalKeepsTheArrayForm checks the round trip.
func TestRichTextValueMarshalKeepsTheArrayForm(t *testing.T) {
	value := RichTextValue{Elements: []RichObject{
		{Type: RichTypeText, TextInfo: &RichTextInfo{Text: "hi"}},
	}}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != `[{"type":1,"text_info":{"text":"hi"}}]` {
		t.Errorf("encoded = %s", encoded)
	}

	plain, err := json.Marshal(RichTextValue{Text: "hi"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(plain) != `"hi"` {
		t.Errorf("encoded = %s, want the string form", plain)
	}
}

// TestRichObjectPlainText covers every documented element type.
func TestRichObjectPlainText(t *testing.T) {
	cases := map[string]struct {
		element RichObject
		want    string
	}{
		"text":    {RichObject{Type: RichTypeText, TextInfo: &RichTextInfo{Text: "文本"}}, "文本"},
		"url":     {RichObject{Type: RichTypeURL, URLInfo: &RichURLInfo{DisplayText: "Apple"}}, "Apple"},
		"emoji":   {RichObject{Type: RichTypeEmoji, EmojiInfo: &RichEmojiInfo{Name: "玫瑰"}}, "玫瑰"},
		"channel": {RichObject{Type: RichTypeChannel, ChannelInfo: &RichChannelInfo{ChannelName: "#综合"}}, "#综合"},
		"at user": {
			RichObject{Type: RichTypeAt, AtInfo: &RichAtInfo{
				Type: AtTypeExplicitUser, UserInfo: &RichAtUserInfo{Nick: "小明"}}}, "@小明"},
		"at role": {
			RichObject{Type: RichTypeAt, AtInfo: &RichAtInfo{
				Type: AtTypeRoleGroup, RoleInfo: &RichAtRoleInfo{Name: "管理员"}}}, "@管理员"},
		"at guild": {
			RichObject{Type: RichTypeAt, AtInfo: &RichAtInfo{
				Type: AtTypeGuild, GuildInfo: &RichAtGuildInfo{GuildName: "读书分享会"}}}, "@读书分享会"},
		"unknown": {RichObject{Type: RichTypeImage}, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.element.PlainText(); got != tc.want {
				t.Errorf("PlainText() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLooseString covers the tolerant id decoder directly.
func TestLooseString(t *testing.T) {
	cases := map[string]string{
		`"123"`:             "123",
		`123`:               "123",
		`47129941624960822`: "47129941624960822",
		`null`:              "",
	}
	for input, want := range cases {
		if got := looseString(json.RawMessage(input)); got != want {
			t.Errorf("looseString(%s) = %q, want %q", input, got, want)
		}
	}
	if got := looseString(nil); got != "" {
		t.Errorf("looseString(nil) = %q, want empty", got)
	}
}

// TestForumEventsAreCovered checks that every forum event name decodes.
func TestForumEventsAreCovered(t *testing.T) {
	bodies := map[string]string{
		EventForumThreadCreate:       `{"guild_id":1,"thread_info":{"thread_id":"T1"}}`,
		EventForumThreadUpdate:       `{"guild_id":1,"thread_info":{"thread_id":"T1"}}`,
		EventForumThreadDelete:       `{"guild_id":1,"thread_info":{"thread_id":"T1"}}`,
		EventForumPostCreate:         `{"guild_id":1,"post_info":{"post_id":"P1"}}`,
		EventForumPostDelete:         `{"guild_id":1,"post_info":{"post_id":"P1"}}`,
		EventForumReplyCreate:        `{"guild_id":1,"reply_info":{"reply_id":"R1"}}`,
		EventForumReplyDelete:        `{"guild_id":1,"reply_info":{"reply_id":"R1"}}`,
		EventForumPublishAuditResult: `{"guild_id":1,"type":1,"result":0}`,
	}
	for eventType, body := range bodies {
		t.Run(eventType, func(t *testing.T) {
			if value, err := DecodeEvent(dispatchPayload(t, eventType, body)); err != nil {
				t.Errorf("DecodeEvent(%s): %v", eventType, err)
			} else if value == nil {
				t.Error("value is nil")
			}
		})
	}
}
