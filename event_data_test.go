package qqbotsdk

import (
	"encoding/json"
	"strings"
	"testing"
)

// dispatchPayload builds a dispatch payload from an event type and a raw body,
// the way the transports deliver one.
func dispatchPayload(t *testing.T, eventType, body string) *Payload {
	t.Helper()
	if !json.Valid([]byte(body)) {
		t.Fatalf("the fixture for %s is not valid JSON", eventType)
	}
	return &Payload{ID: "EVENT1", Op: OpDispatch, Type: eventType, Data: json.RawMessage(body)}
}

// TestDecodeFriendAdd reproduces the documented response example exactly.
func TestDecodeFriendAdd(t *testing.T) {
	payload := dispatchPayload(t, EventFriendAdd, `{
		"openid": "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4",
		"timestamp": 1784570523,
		"scene": 1001,
		"scene_param": "",
		"author": {"union_openid": "DB85A74E07BA08B5B44CD9ED332FCBD2"}
	}`)

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	data, ok := value.(*FriendAddData)
	if !ok {
		t.Fatalf("value = %T, want *FriendAddData", value)
	}

	if data.OpenID != "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4" {
		t.Errorf("OpenID = %q", data.OpenID)
	}
	if data.Timestamp != 1784570523 {
		t.Errorf("Timestamp = %d", data.Timestamp)
	}
	if data.Scene != FriendSceneSearchAll {
		t.Errorf("Scene = %d, want the search scene", data.Scene)
	}
	if data.Author == nil || data.Author.UnionOpenID != "DB85A74E07BA08B5B44CD9ED332FCBD2" {
		t.Errorf("Author = %+v", data.Author)
	}
}

// TestDecodeFriendAddShareLink covers the documented share link scene, which
// carries the developer callback data.
func TestDecodeFriendAddShareLink(t *testing.T) {
	payload := dispatchPayload(t, EventFriendAdd, `{
		"openid": "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4",
		"timestamp": 1784570600,
		"scene": 2003,
		"scene_param": "callback_abc123",
		"author": {"union_openid": "DB85A74E07BA08B5B44CD9ED332FCBD2"}
	}`)

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	data := value.(*FriendAddData)
	if data.Scene != FriendSceneShareLinkInApp {
		t.Errorf("Scene = %d, want the share link scene", data.Scene)
	}
	if data.SceneParam != "callback_abc123" {
		t.Errorf("SceneParam = %q, want the callback data", data.SceneParam)
	}
}

// TestDecodeC2CMessageCreateText reproduces documented example 1.
func TestDecodeC2CMessageCreateText(t *testing.T) {
	payload := dispatchPayload(t, EventC2CMessageCreate, `{
		"id": "ROBOT1.0_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"author": {
			"id": "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4",
			"user_openid": "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4",
			"union_openid": "",
			"username": "",
			"bot": false
		},
		"content": "你好，今天有什么推荐的活动吗？",
		"message_type": 0,
		"message_scene": {"source": "default", "ext": ["msg_idx=REFIDX_xxxxxxxxxxxxxxx=="]},
		"timestamp": "2026-07-21T10:00:00+08:00"
	}`)

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	data, ok := value.(*C2CMessageCreateData)
	if !ok {
		t.Fatalf("value = %T, want *C2CMessageCreateData", value)
	}

	if data.Content != "你好，今天有什么推荐的活动吗？" {
		t.Errorf("Content = %q", data.Content)
	}
	if data.MessageType != EventMsgTypeText {
		t.Errorf("MessageType = %d, want text", data.MessageType)
	}
	if data.Author == nil || data.Author.UserOpenID == "" {
		t.Errorf("Author = %+v, want the user openid", data.Author)
	}
	if data.MessageScene == nil {
		t.Fatal("MessageScene is nil")
	}
	if idx, ok := data.MessageScene.MsgIdx(); !ok || idx != "REFIDX_xxxxxxxxxxxxxxx==" {
		t.Errorf("MsgIdx = %q, %v", idx, ok)
	}
}

// TestDecodeC2CMessageCreateArk reproduces documented example 2, a mini app
// card.
func TestDecodeC2CMessageCreateArk(t *testing.T) {
	payload := dispatchPayload(t, EventC2CMessageCreate, `{
		"id": "ROBOT1.0_yyyy",
		"author": {"id": "B2", "user_openid": "B2", "bot": false},
		"content": "[卡片消息] 小程序",
		"message_type": 3,
		"ark_data": {
			"ark_type": "miniapp",
			"ark_name": "小程序",
			"prompt": "[每日打卡]快来完成今日学习打卡",
			"fields": {"title": "快来完成今日学习打卡", "source": "学习助手"}
		},
		"message_scene": {"source": "default", "ext": ["msg_idx=REFIDX_y=="]}
	}`)

	data := mustDecode(t, payload).(*C2CMessageCreateData)
	if data.MessageType != EventMsgTypeArk {
		t.Errorf("MessageType = %d, want the card type", data.MessageType)
	}
	if data.ArkData == nil {
		t.Fatal("ArkData is nil")
	}
	if data.ArkData.ArkType != ARKTypeMiniApp || data.ArkData.ArkName != "小程序" {
		t.Errorf("ArkData = %+v", data.ArkData)
	}
	if data.ArkData.Fields["title"] != "快来完成今日学习打卡" {
		t.Errorf("Fields = %v", data.ArkData.Fields)
	}
}

// TestDecodeC2CMessageCreateQuote reproduces documented example 3, a quote.
func TestDecodeC2CMessageCreateQuote(t *testing.T) {
	payload := dispatchPayload(t, EventC2CMessageCreate, `{
		"id": "ROBOT1.0_zzz",
		"author": {"id": "C3", "user_openid": "C3", "bot": false},
		"content": "这个建议很有帮助，谢谢你！",
		"message_type": 103,
		"msg_elements": [{
			"msg_idx": "REFIDX_aaaaaaaaaaaaaaa==",
			"message_type": 103,
			"content": "每天坚持阅读半小时，一个月后你会发现自己的变化"
		}],
		"message_scene": {"source": "default",
			"ext": ["ref_msg_idx=REFIDX_aaaaaaaaaaaaaaa==", "msg_idx=REFIDX_zzz=="]}
	}`)

	data := mustDecode(t, payload).(*C2CMessageCreateData)
	if data.MessageType != EventMsgTypeQuote {
		t.Errorf("MessageType = %d, want the quote type", data.MessageType)
	}
	if len(data.MsgElements) != 1 {
		t.Fatalf("MsgElements = %d, want 1", len(data.MsgElements))
	}
	if data.MsgElements[0].Content == "" || data.MsgElements[0].MsgIdx == "" {
		t.Errorf("element = %+v", data.MsgElements[0])
	}

	ref, ok := data.MessageScene.RefMsgIdx()
	if !ok || ref != "REFIDX_aaaaaaaaaaaaaaa==" {
		t.Errorf("RefMsgIdx = %q, %v", ref, ok)
	}
}

// TestDecodeGroupAtMessageCreate covers the nested mention list.
func TestDecodeGroupAtMessageCreate(t *testing.T) {
	payload := dispatchPayload(t, EventGroupAtMessageCreate, `{
		"id": "ROBOT1.0_group",
		"author": {"id": "M1", "member_openid": "M1", "member_role": "admin", "bot": false},
		"content": "帮我看看这个",
		"group_openid": "GROUP1",
		"timestamp": "2026-07-21T10:00:00+08:00",
		"message_type": 0,
		"mentions": [{"id": "M2", "member_openid": "M2", "username": "小明"}],
		"attachments": [{"url": "https://example.com/a.png", "content_type": "image/png",
			"width": 100, "height": 50, "size": 2048, "filename": "a.png"}]
	}`)

	data := mustDecode(t, payload).(*GroupMessageCreateData)
	if data.GroupOpenID != "GROUP1" {
		t.Errorf("GroupOpenID = %q", data.GroupOpenID)
	}
	if data.Author == nil || data.Author.MemberRole != GroupMemberRoleAdmin {
		t.Errorf("Author = %+v", data.Author)
	}
	if len(data.Mentions) != 1 || data.Mentions[0].Username != "小明" {
		t.Errorf("Mentions = %+v", data.Mentions)
	}
	if len(data.Attachments) != 1 {
		t.Fatalf("Attachments = %d, want 1", len(data.Attachments))
	}
	attachment := data.Attachments[0]
	if !attachment.IsImage() || attachment.IsVoice() {
		t.Errorf("attachment = %+v, want an image", attachment)
	}
	if attachment.Width != 100 || attachment.Height != 50 || attachment.Size != 2048 {
		t.Errorf("attachment = %+v", attachment)
	}
}

// TestDecodeGroupMessageCreateFullMode checks the full mode event, which shares
// the structure of the mention event.
func TestDecodeGroupMessageCreateFullMode(t *testing.T) {
	payload := dispatchPayload(t, EventGroupMessageCreate, `{
		"id": "ROBOT1.0_full",
		"author": {"id": "M3", "member_openid": "M3"},
		"content": "普通消息",
		"group_openid": "GROUP2",
		"message_type": 0
	}`)

	if _, ok := mustDecode(t, payload).(*GroupMessageCreateData); !ok {
		t.Error("GROUP_MESSAGE_CREATE must decode into the group message structure")
	}
}

// TestDecodeVoiceAttachment checks the documented voice fields.
func TestDecodeVoiceAttachment(t *testing.T) {
	payload := dispatchPayload(t, EventC2CMessageCreate, `{
		"id": "MSG1", "author": {"id": "U1", "user_openid": "U1"}, "message_type": 0,
		"attachments": [{"url": "https://example.com/v.silk", "content_type": "voice",
			"voice_wav_url": "https://example.com/v.wav", "asr_refer_text": "你好"}]
	}`)

	attachment := mustDecode(t, payload).(*C2CMessageCreateData).Attachments[0]
	if !attachment.IsVoice() || attachment.IsImage() {
		t.Errorf("attachment = %+v, want voice", attachment)
	}
	if attachment.VoiceWavURL == "" || attachment.ASRReferText != "你好" {
		t.Errorf("attachment = %+v", attachment)
	}
}

// TestDecodeGroupJoinRequest reproduces the documented join request body.
func TestDecodeGroupJoinRequest(t *testing.T) {
	payload := dispatchPayload(t, EventGroupJoinRequest, `{
		"group_openid": "GROUP1",
		"join_request_id": "REQ1",
		"risk_tips": "warning_tips",
		"union_openid": "U1",
		"member_openid": "M1",
		"username": "小明",
		"apply_at": "2026-07-21T10:00:00+08:00",
		"apply_source": "invited",
		"invited_by": "M9",
		"bot": false,
		"verify_info": {
			"method": "admin_review_qa",
			"review_qa_list": [{"question": "你的学号？", "answer": "12345"}]
		},
		"auto_approved": {"strategy_id": "S1"}
	}`)

	data := mustDecode(t, payload).(*GroupJoinRequestData)
	if data.JoinRequestID != "REQ1" || data.ApplySource != GroupJoinSourceInvited {
		t.Errorf("data = %+v", data)
	}
	if data.InvitedBy != "M9" {
		t.Errorf("InvitedBy = %q", data.InvitedBy)
	}
	if data.VerifyInfo == nil || data.VerifyInfo.Method != GroupVerifyAdminReviewQA {
		t.Fatalf("VerifyInfo = %+v", data.VerifyInfo)
	}
	if len(data.VerifyInfo.ReviewQAList) != 1 || data.VerifyInfo.ReviewQAList[0].Answer != "12345" {
		t.Errorf("ReviewQAList = %+v", data.VerifyInfo.ReviewQAList)
	}
	if data.AutoApproved == nil || data.AutoApproved.StrategyID != "S1" {
		t.Errorf("AutoApproved = %+v", data.AutoApproved)
	}
}

// TestDecodeSubscribeMessageStatus covers the per template authorization list.
func TestDecodeSubscribeMessageStatus(t *testing.T) {
	payload := dispatchPayload(t, EventSubscribeMsgStatus, `{
		"openid": "U1",
		"result": [{
			"template_id": 1001,
			"custom_template_id": "c1",
			"op": 1,
			"subscribe_id": "S1",
			"subscribe_ts": 1784570523,
			"update_ts": 1784570600
		}]
	}`)

	data := mustDecode(t, payload).(*SubscribeMessageStatusData)
	if data.OpenID != "U1" || data.GroupOpenID != "" {
		t.Errorf("data = %+v, want the personal subscription", data)
	}
	if len(data.Result) != 1 {
		t.Fatalf("Result = %d, want 1", len(data.Result))
	}
	result := data.Result[0]
	if result.TemplateID != 1001 || result.Op != SubscribeOpAllowed {
		t.Errorf("result = %+v", result)
	}
	if result.SubscribeID != "S1" || result.UpdateTS != 1784570600 {
		t.Errorf("result = %+v", result)
	}
}

// TestDecodeGuildEvents covers the shared guild structure.
func TestDecodeGuildEvents(t *testing.T) {
	body := `{
		"id": "G1", "name": "读书分享会", "icon": "https://example.com/i.png",
		"owner_id": "O1", "member_count": 6, "max_members": 5000000,
		"description": "一起读书", "joined_at": "2025-01-09T15:17:23+08:00",
		"op_user_id": "OP1"
	}`

	for _, eventType := range []string{EventGuildCreate, EventGuildUpdate, EventGuildDelete} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*GuildInfo)
			if data.ID != "G1" || data.Name != "读书分享会" {
				t.Errorf("data = %+v", data)
			}
			if data.OpUserID != "OP1" {
				t.Errorf("OpUserID = %q, want the operator", data.OpUserID)
			}
			if data.MemberCount != 6 || data.MaxMembers != 5000000 {
				t.Errorf("data = %+v", data)
			}
		})
	}
}

// TestDecodeChannelEvents covers the shared channel structure.
func TestDecodeChannelEvents(t *testing.T) {
	body := `{
		"id": "C1", "guild_id": "G1", "name": "综合", "type": 10007,
		"sub_type": 1, "owner_id": "O1", "op_user_id": "OP1"
	}`

	for _, eventType := range []string{EventChannelCreate, EventChannelUpdate, EventChannelDelete} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*ChannelInfo)
			if data.ID != "C1" || data.GuildID != "G1" {
				t.Errorf("data = %+v", data)
			}
			if data.Type != ChannelTypeForum {
				t.Errorf("Type = %d, want the forum type", data.Type)
			}
		})
	}
}

// TestDecodeSimpleGroupEvents covers the small events that only carry a
// timestamp and identifiers, so a field typo cannot slip through unnoticed.
func TestDecodeSimpleGroupEvents(t *testing.T) {
	cases := []struct {
		eventType string
		body      string
		check     func(t *testing.T, value any)
	}{
		{EventC2CMsgReceive, `{"timestamp":1784570523,"openid":"U1"}`,
			func(t *testing.T, v any) {
				if d := v.(*C2CMsgReceiveData); d.OpenID != "U1" || d.Timestamp != 1784570523 {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventC2CMsgReject, `{"timestamp":1784570523,"openid":"U1"}`,
			func(t *testing.T, v any) {
				if d := v.(*C2CMsgRejectData); d.OpenID != "U1" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventFriendDel, `{"timestamp":1,"openid":"U1","author":{"union_openid":"X"}}`,
			func(t *testing.T, v any) {
				d := v.(*FriendDelData)
				if d.OpenID != "U1" || d.Author == nil || d.Author.UnionOpenID != "X" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupAddRobot, `{"timestamp":1,"group_openid":"G1","op_member_openid":"M1"}`,
			func(t *testing.T, v any) {
				d := v.(*GroupAddRobotData)
				if d.GroupOpenID != "G1" || d.OpMemberOpenID != "M1" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupDelRobot, `{"timestamp":1,"group_openid":"G1","op_member_openid":"M1"}`,
			func(t *testing.T, v any) {
				if d := v.(*GroupDelRobotData); d.GroupOpenID != "G1" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupMemberAdd, `{"timestamp":1,"group_openid":"G1","member_openid":"M2","user_openid":"U2"}`,
			func(t *testing.T, v any) {
				d := v.(*GroupMemberAddData)
				if d.MemberOpenID != "M2" || d.UserOpenID != "U2" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupMemberRemove, `{"timestamp":1,"group_openid":"G1","member_openid":"M2"}`,
			func(t *testing.T, v any) {
				if d := v.(*GroupMemberRemoveData); d.MemberOpenID != "M2" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupMsgReceive, `{"timestamp":1,"group_openid":"G1","op_member_openid":"M1"}`,
			func(t *testing.T, v any) {
				if d := v.(*GroupMsgReceiveData); d.OpMemberOpenID != "M1" {
					t.Errorf("data = %+v", d)
				}
			}},
		{EventGroupMsgReject, `{"timestamp":1,"group_openid":"G1","op_member_openid":"M1"}`,
			func(t *testing.T, v any) {
				if d := v.(*GroupMsgRejectData); d.GroupOpenID != "G1" {
					t.Errorf("data = %+v", d)
				}
			}},
	}

	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			tc.check(t, mustDecode(t, dispatchPayload(t, tc.eventType, tc.body)))
		})
	}
}

// TestDecodeInteractionCreate reproduces the documented interaction body,
// including the nested resolved payload.
func TestDecodeInteractionCreate(t *testing.T) {
	payload := dispatchPayload(t, EventInteractionCreate, `{
		"id": "INTER1",
		"type": 11,
		"scene": "group",
		"chat_type": 1,
		"timestamp": "2026-07-21T10:00:00+08:00",
		"group_openid": "GROUP1",
		"group_member_openid": "M1",
		"data": {
			"type": 11,
			"resolved": {
				"button_data": "/vote yes",
				"button_id": "btn1",
				"message_id": "MSG1"
			}
		},
		"version": 1,
		"application_id": "102083127"
	}`)

	data := mustDecode(t, payload).(*InteractionCreateData)
	if data.Scene != InteractionSceneGroup || data.ChatType != InteractionChatGroup {
		t.Errorf("data = %+v", data)
	}
	if data.GroupOpenID != "GROUP1" || data.ApplicationID != "102083127" {
		t.Errorf("data = %+v", data)
	}
	if data.Data == nil || data.Data.Resolved == nil {
		t.Fatalf("Data = %+v", data.Data)
	}
	if data.Data.Resolved.ButtonData != "/vote yes" || data.Data.Resolved.ButtonID != "btn1" {
		t.Errorf("Resolved = %+v", data.Data.Resolved)
	}

	// A message button and a custom menu must be answered.
	if !data.NeedsResponse() {
		t.Error("a message button interaction must need a response")
	}
}

// TestInteractionNeedsResponse pins the documented rule: only the message
// button and the custom menu require an answer.
func TestInteractionNeedsResponse(t *testing.T) {
	needs := map[int]bool{
		InteractionInlineKeyboard:       true,
		InteractionCallbackCommand:      true,
		InteractionMessageFeedback:      false,
		InteractionClearSession:         false,
		InteractionInOutStory:           false,
		InteractionSwitchModel:          false,
		InteractionUserAuthorize:        false,
		InteractionGroupAuthorize:       false,
		InteractionGroupAuthorizeStatus: false,
	}
	for interactionType, want := range needs {
		data := &InteractionCreateData{Type: interactionType}
		if got := data.NeedsResponse(); got != want {
			t.Errorf("type %d NeedsResponse() = %v, want %v", interactionType, got, want)
		}
	}

	if (*InteractionCreateData)(nil).NeedsResponse() {
		t.Error("a nil interaction must not need a response")
	}
}

// TestDecodeInteractionResolvedVariants covers the fields that only some
// interaction types carry.
func TestDecodeInteractionResolvedVariants(t *testing.T) {
	feedback := mustDecode(t, dispatchPayload(t, EventInteractionCreate, `{
		"id":"I1","type":13,"scene":"c2c","chat_type":2,
		"data":{"type":13,"resolved":{
			"feedback_opt":"LIKE","checked":1,"message_id":"MSG1",
			"message_scene":{"ext":["disable_net_search=1"]}}}
	}`)).(*InteractionCreateData)
	if feedback.NeedsResponse() {
		t.Error("a feedback interaction must not need a response")
	}
	resolved := feedback.Data.Resolved
	if resolved.FeedbackOpt != InteractionFeedbackLike || resolved.Checked != 1 {
		t.Errorf("resolved = %+v", resolved)
	}
	if resolved.MessageScene == nil || len(resolved.MessageScene.Ext) != 1 {
		t.Errorf("MessageScene = %+v", resolved.MessageScene)
	}

	authorize := mustDecode(t, dispatchPayload(t, EventInteractionCreate, `{
		"id":"I2","type":19,"scene":"group","chat_type":1,
		"data":{"type":19,"resolved":{"authorize_data":{"opt_scene":"dialog","scope":"group_push"}}}
	}`)).(*InteractionCreateData)
	auth := authorize.Data.Resolved.AuthorizeData
	if auth == nil || auth.OptScene != InteractionAuthSceneDialog || auth.Scope != InteractionAuthScopeGroupPush {
		t.Errorf("AuthorizeData = %+v", auth)
	}

	story := mustDecode(t, dispatchPayload(t, EventInteractionCreate, `{
		"id":"I3","type":15,"scene":"c2c","chat_type":2,
		"data":{"type":15,"resolved":{"action":"ENTER_STORY"}}
	}`)).(*InteractionCreateData)
	if got := story.Data.Resolved.Action; got != InteractionEnterStory {
		t.Errorf("Action = %q", got)
	}
}

// TestEventDataForCoversDocumentedEvents checks that every event type the
// autogen documentation describes has a structure.
func TestEventDataForCoversDocumentedEvents(t *testing.T) {
	documented := []string{
		EventC2CMessageCreate, EventC2CMsgReceive, EventC2CMsgReject,
		EventChannelCreate, EventChannelDelete, EventChannelUpdate,
		EventFriendAdd, EventFriendDel, EventGroupAddRobot, EventGroupAtMessageCreate,
		EventGroupDelRobot, EventGroupJoinRequest, EventGroupMemberAdd,
		EventGroupMemberRemove, EventGroupMessageCreate, EventGroupMsgReceive,
		EventGroupMsgReject, EventGuildCreate, EventGuildDelete, EventGuildUpdate,
		EventInteractionCreate, EventSubscribeMsgStatus,
	}
	for _, eventType := range documented {
		if target := EventDataFor(eventType); target == nil {
			t.Errorf("%s has no structure", eventType)
		}
	}

	if EventDataFor("NOT_AN_EVENT") != nil {
		t.Error("an unknown event type must return nil")
	}
}

// TestIntentForNewEvents covers the intent table entries added with these
// events.
func TestIntentForNewEvents(t *testing.T) {
	cases := map[string]Intent{
		EventGroupMemberAdd:     IntentGroupMemberEvent,
		EventGroupMemberRemove:  IntentGroupMemberEvent,
		EventGroupJoinRequest:   IntentGroupMemberEvent,
		EventGroupMessageCreate: IntentGroupAndC2CEvent,
		EventSubscribeMsgStatus: IntentGroupAndC2CEvent,
	}
	for eventType, want := range cases {
		if got := IntentForEvent(eventType); got != want {
			t.Errorf("IntentForEvent(%s) = %d, want %d", eventType, got, want)
		}
	}

	if got := Intent(1 << 24); got != IntentGroupMemberEvent {
		t.Errorf("1<<24 = %d, want IntentGroupMemberEvent", got)
	}
}

// TestDecodeEventErrors covers the failure modes of the decoder.
func TestDecodeEventErrors(t *testing.T) {
	if _, err := DecodeEvent(nil); err == nil {
		t.Error("a nil payload must be reported")
	}

	hello := &Payload{Op: OpHello}
	if _, err := DecodeEvent(hello); err == nil {
		t.Error("a non dispatch opcode must be reported")
	} else if !strings.Contains(err.Error(), "Hello") {
		t.Errorf("err = %v, want it to name the opcode", err)
	}

	unknown := dispatchPayload(t, "SOME_FUTURE_EVENT", `{}`)
	if _, err := DecodeEvent(unknown); err == nil {
		t.Error("an event without a structure must be reported")
	} else if !strings.Contains(err.Error(), "SOME_FUTURE_EVENT") {
		t.Errorf("err = %v, want it to name the event type", err)
	}

	// A body that does not fit its structure must be reported, not ignored.
	mismatch := dispatchPayload(t, EventFriendAdd, `{"timestamp":"not-a-number"}`)
	if _, err := DecodeEvent(mismatch); err == nil {
		t.Error("a mismatched body must be reported")
	}
}

// mustDecode decodes a payload and fails the test on error.
func mustDecode(t *testing.T, payload *Payload) any {
	t.Helper()
	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent(%s): %v", payload.Type, err)
	}
	return value
}

// TestEveryEventHasAStructure requires every event type name to have a body
// structure, so a newly added event name cannot silently be left undecodable.
//
// Two of the structures are inferred rather than spelled out on an event page,
// and both are marked as such where they are declared: AudioAction for the four
// AUDIO_* events, and MemberWithGuildID for the three GUILD_MEMBER_* events.
// Each is a documented object that no endpoint uses, which is what identifies
// it as the event body.
func TestEveryEventHasAStructure(t *testing.T) {
	allEvents := []string{
		"AT_MESSAGE_CREATE", "AUDIO_FINISH", "AUDIO_OFF_MIC", "AUDIO_ON_MIC", "AUDIO_START",
		"CHANNEL_CREATE", "CHANNEL_DELETE", "CHANNEL_UPDATE",
		"DIRECT_MESSAGE_CREATE", "DIRECT_MESSAGE_DELETE",
		"FORUM_POST_CREATE", "FORUM_POST_DELETE", "FORUM_PUBLISH_AUDIT_RESULT",
		"FORUM_REPLY_CREATE", "FORUM_REPLY_DELETE",
		"FORUM_THREAD_CREATE", "FORUM_THREAD_DELETE", "FORUM_THREAD_UPDATE",
		"FRIEND_ADD", "FRIEND_DEL",
		"GROUP_ADD_ROBOT", "GROUP_AT_MESSAGE_CREATE", "GROUP_DEL_ROBOT",
		"GROUP_JOIN_REQUEST", "GROUP_MEMBER_ADD", "GROUP_MEMBER_REMOVE",
		"GROUP_MESSAGE_CREATE", "GROUP_MSG_RECEIVE", "GROUP_MSG_REJECT",
		"GUILD_CREATE", "GUILD_DELETE", "GUILD_MEMBER_ADD", "GUILD_MEMBER_REMOVE",
		"GUILD_MEMBER_UPDATE", "GUILD_UPDATE", "INTERACTION_CREATE",
		"MESSAGE_AUDIT_PASS", "MESSAGE_AUDIT_REJECT", "MESSAGE_CREATE", "MESSAGE_DELETE",
		"MESSAGE_REACTION_ADD", "MESSAGE_REACTION_REMOVE", "PUBLIC_MESSAGE_DELETE",
		"READY", "RESUMED", "SUBSCRIBE_MESSAGE_STATUS",
	}

	for _, eventType := range allEvents {
		if EventDataFor(eventType) == nil {
			t.Errorf("%s has no body structure", eventType)
		}
	}
	if len(allEvents) != 46 {
		t.Errorf("allEvents has %d entries, want the 46 documented event types", len(allEvents))
	}
}

// TestEventDecodeShorthand covers the method a handler uses directly, since it
// already holds the event.
func TestEventDecodeShorthand(t *testing.T) {
	event := &Event{
		Payload: dispatchPayload(t, EventGroupAtMessageCreate, `{
			"id":"M1","content":"你好","group_openid":"G1",
			"author":{"member_openid":"U1"}
		}`),
		Transport: TransportNameWebSocket,
	}

	value, err := event.Decode()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	data, ok := value.(*GroupMessageCreateData)
	if !ok {
		t.Fatalf("value = %T, want *GroupMessageCreateData", value)
	}
	if data.Content != "你好" || data.GroupOpenID != "G1" {
		t.Errorf("data = %+v", data)
	}
	if data.Author == nil || data.Author.MemberOpenID != "U1" {
		t.Errorf("Author = %+v", data.Author)
	}

	if _, err := (*Event)(nil).Decode(); err == nil {
		t.Error("a nil event must be reported")
	}
	if _, err := (&Event{}).Decode(); err == nil {
		t.Error("an event without a payload must be reported")
	}
}
