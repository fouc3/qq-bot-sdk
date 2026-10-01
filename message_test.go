package qqbotsdk

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// capturedRequest is one request the SDK sent to a test server.
type capturedRequest struct {
	Method string
	Path   string
	// EscapedPath keeps the percent-encoding, so a test can check that an id
	// cannot alter the route.
	EscapedPath string
	Query       string
	ContentType string
	Body        []byte
}

// newMessageServer returns a client pointed at a test server that records every
// request and replies with the given JSON body.
func newMessageServer(t *testing.T, status int, response string) (*Client, *[]capturedRequest) {
	t.Helper()

	var captured []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured = append(captured, capturedRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			EscapedPath: r.URL.EscapedPath(),
			Query:       r.URL.RawQuery,
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != "" {
			_, _ = io.WriteString(w, response)
		}
	}))
	t.Cleanup(srv.Close)

	client := NewClientFromConfigMust(t, Config{AccessToken: "T", BaseURL: srv.URL})
	return client, &captured
}

// last returns the most recent captured request.
func last(t *testing.T, captured *[]capturedRequest) capturedRequest {
	t.Helper()
	if len(*captured) == 0 {
		t.Fatal("no request was sent")
	}
	return (*captured)[len(*captured)-1]
}

// decodeBody unmarshals a captured body into a map.
func decodeBody(t *testing.T, req capturedRequest) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(req.Body, &got); err != nil {
		t.Fatalf("decode request body %s: %v", req.Body, err)
	}
	return got
}

// TestSendC2CMessageMatchesDocumentedRequest reproduces the documented text
// message request.
func TestSendC2CMessageMatchesDocumentedRequest(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"id":"ROBOT1.0_msg","timestamp":"2026-07-21T10:30:00+08:00"}`)

	resp, err := client.SendC2CMessage(t.Context(), "A1B2C3D4", &Message{
		Content: "你好，欢迎使用机器人助手！",
		MsgType: MsgTypeText,
		MsgID:   "ROBOT1.0_xxx",
		MsgSeq:  1,
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/v2/users/A1B2C3D4/messages" {
		t.Errorf("path = %s, want the documented address", req.Path)
	}

	body := decodeBody(t, req)
	if body["content"] != "你好，欢迎使用机器人助手！" {
		t.Errorf("content = %v", body["content"])
	}
	if body["msg_id"] != "ROBOT1.0_xxx" {
		t.Errorf("msg_id = %v", body["msg_id"])
	}
	if body["msg_seq"] != float64(1) {
		t.Errorf("msg_seq = %v, want 1", body["msg_seq"])
	}
	// msg_type 0 is the documented value for plain text; omitting it is
	// equivalent because the platform defaults to text.
	if _, present := body["msg_type"]; present {
		t.Errorf("msg_type = %v, want it omitted for the zero value", body["msg_type"])
	}

	if resp.ID != "ROBOT1.0_msg" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.Timestamp != "2026-07-21T10:30:00+08:00" {
		t.Errorf("Timestamp = %q", resp.Timestamp)
	}
}

// TestSendC2CMessageMarkdown checks the markdown body shape and the msg_type
// value that selects it.
func TestSendC2CMessageMarkdown(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{
		MsgType:  MsgTypeMarkdown,
		Markdown: &MessageMarkdown{Content: "# 今日推荐"},
		Keyboard: &Keyboard{ID: "1070001"},
		MsgID:    "msg-1",
		MsgSeq:   1,
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	if body["msg_type"] != float64(MsgTypeMarkdown) {
		t.Errorf("msg_type = %v, want %d", body["msg_type"], MsgTypeMarkdown)
	}
	markdown, ok := body["markdown"].(map[string]any)
	if !ok {
		t.Fatalf("markdown = %T, want an object", body["markdown"])
	}
	if markdown["content"] != "# 今日推荐" {
		t.Errorf("markdown.content = %v", markdown["content"])
	}
	keyboard, ok := body["keyboard"].(map[string]any)
	if !ok || keyboard["id"] != "1070001" {
		t.Errorf("keyboard = %v, want the template id", body["keyboard"])
	}
}

// TestSendC2CMessageKeyboardLayout covers the custom keyboard structure.
func TestSendC2CMessageKeyboardLayout(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{
		MsgType:  MsgTypeMarkdown,
		Markdown: &MessageMarkdown{Content: "x"},
		Keyboard: &Keyboard{Content: &KeyboardContent{Rows: []Row{{
			Buttons: []Button{{
				ID:         "btn_signin",
				RenderData: &RenderData{Label: "签到", Style: KeyboardStyleBlue},
				Action: &Action{
					Type:       ActionTypeCommand,
					Data:       "/签到",
					Enter:      true,
					Permission: &Permission{Type: PermissionTypeEveryone},
				},
			}},
		}}}},
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	keyboard := body["keyboard"].(map[string]any)
	rows := keyboard["content"].(map[string]any)["rows"].([]any)
	buttons := rows[0].(map[string]any)["buttons"].([]any)
	button := buttons[0].(map[string]any)

	if button["id"] != "btn_signin" {
		t.Errorf("button id = %v", button["id"])
	}
	render := button["render_data"].(map[string]any)
	if render["label"] != "签到" || render["style"] != float64(KeyboardStyleBlue) {
		t.Errorf("render_data = %v", render)
	}
	action := button["action"].(map[string]any)
	if action["type"] != float64(ActionTypeCommand) || action["data"] != "/签到" || action["enter"] != true {
		t.Errorf("action = %v", action)
	}
}

// TestSendC2CMessageMedia checks the msg_type=7 body.
func TestSendC2CMessageMedia(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{
		MsgType: MsgTypeMedia,
		Media:   &MediaInfo{FileInfo: "AE86C5D3F0E14B238C656C0F6DD1D0479C"},
		MsgID:   "msg-1",
		MsgSeq:  1,
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	if body["msg_type"] != float64(MsgTypeMedia) {
		t.Errorf("msg_type = %v, want %d", body["msg_type"], MsgTypeMedia)
	}
	media := body["media"].(map[string]any)
	if media["file_info"] != "AE86C5D3F0E14B238C656C0F6DD1D0479C" {
		t.Errorf("media.file_info = %v", media["file_info"])
	}
}

// TestSendC2CMessageInputNotify covers the typing state message.
func TestSendC2CMessageInputNotify(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{
		MsgType:     MsgTypeInputNotify,
		InputNotify: &InputNotify{InputType: 1, InputSecond: 60},
		MsgID:       "msg-1",
		MsgSeq:      1,
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	notify := body["input_notify"].(map[string]any)
	if notify["input_type"] != float64(1) || notify["input_second"] != float64(60) {
		t.Errorf("input_notify = %v", notify)
	}
}

// TestSendC2CMessageReference covers the quote field.
func TestSendC2CMessageReference(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{
		Content:          "reply",
		MessageReference: &MessageReference{MessageID: "REFIDX_abc"},
	})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	reference := body["message_reference"].(map[string]any)
	if reference["message_id"] != "REFIDX_abc" {
		t.Errorf("message_reference = %v", reference)
	}
}

// TestSendC2CMessageExtInfo checks the quote index of the response.
func TestSendC2CMessageExtInfo(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"id":"m1","timestamp":"t","ext_info":{"ref_idx":"REFIDX_xyz=="}}`)

	resp, err := client.SendC2CMessage(t.Context(), "USER1", &Message{Content: "x"})
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}
	if resp.ExtInfo == nil || resp.ExtInfo.RefIdx != "REFIDX_xyz==" {
		t.Errorf("ExtInfo = %+v, want the ref index", resp.ExtInfo)
	}
}

func TestSendGroupMessage(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"g1"}`)

	_, err := client.SendGroupMessage(t.Context(), "GROUP1", &Message{
		MsgType: MsgTypeText,
		Content: "欢迎使用本群助手",
		MsgID:   "msg-1",
		MsgSeq:  1,
	})
	if err != nil {
		t.Fatalf("SendGroupMessage: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/v2/groups/GROUP1/messages" {
		t.Errorf("path = %s, want the documented address", req.Path)
	}
	if body := decodeBody(t, req); body["content"] != "欢迎使用本群助手" {
		t.Errorf("content = %v", body["content"])
	}
}

// TestSendC2CStreamMessage covers the three documented streaming chunks.
func TestSendC2CStreamMessage(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"id":"a1b2c3d4","timestamp":"2026-07-21T10:00:00+08:00","remain_msg_len":100}`)

	first, err := client.SendC2CStreamMessage(t.Context(), "USER1", &StreamMessage{
		InputMode:   StreamInputReplace,
		InputState:  StreamInputGenerating,
		Index:       0,
		ContentType: StreamContentMarkdown,
		ContentRaw:  "正在生成回答",
		MsgID:       "msg-1",
		MsgSeq:      1,
	})
	if err != nil {
		t.Fatalf("first chunk: %v", err)
	}
	if first.ID != "a1b2c3d4" {
		t.Errorf("ID = %q, want the stream id to reuse", first.ID)
	}

	req := last(t, captured)
	if req.Path != "/v2/users/USER1/stream_messages" {
		t.Errorf("path = %s, want the stream endpoint", req.Path)
	}
	body := decodeBody(t, req)
	if body["input_mode"] != StreamInputReplace || body["content_type"] != StreamContentMarkdown {
		t.Errorf("body = %v", body)
	}
	if body["input_state"] != float64(StreamInputGenerating) {
		t.Errorf("input_state = %v, want %d", body["input_state"], StreamInputGenerating)
	}
	// The first chunk must not carry a stream_msg_id.
	if _, present := body["stream_msg_id"]; present {
		t.Error("the first chunk must omit stream_msg_id")
	}

	// The last chunk reuses the returned id and reports the finished state.
	last, err := client.SendC2CStreamMessage(t.Context(), "USER1", &StreamMessage{
		InputMode:   StreamInputReplace,
		InputState:  StreamInputFinished,
		Index:       2,
		ContentType: StreamContentMarkdown,
		ContentRaw:  "回答结束",
		MsgID:       "msg-1",
		StreamMsgID: first.ID,
		MsgSeq:      1,
	})
	if err != nil {
		t.Fatalf("last chunk: %v", err)
	}
	if last.RemainMsgLen != 100 {
		t.Errorf("RemainMsgLen = %d, want 100", last.RemainMsgLen)
	}

	body = decodeBody(t, lastRequest(t, captured))
	if body["stream_msg_id"] != "a1b2c3d4" {
		t.Errorf("stream_msg_id = %v, want the first chunk id", body["stream_msg_id"])
	}
	if body["input_state"] != float64(StreamInputFinished) {
		t.Errorf("input_state = %v, want %d", body["input_state"], StreamInputFinished)
	}
}

// lastRequest is last, named for readability at some call sites.
func lastRequest(t *testing.T, captured *[]capturedRequest) capturedRequest {
	t.Helper()
	return last(t, captured)
}

func TestSendChannelMessage(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"id":"c1","channel_id":"123","guild_id":"456","content":"hi","timestamp":"t","author":{"id":"1","username":"abc","bot":true}}`)

	resp, err := client.SendChannelMessage(t.Context(), "123", &ChannelMessage{
		Content: "<@!1234>hello world",
		MsgID:   "xxxxxx",
	})
	if err != nil {
		t.Fatalf("SendChannelMessage: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/channels/123/messages" {
		t.Errorf("path = %s, want the documented address", req.Path)
	}
	if body := decodeBody(t, req); body["content"] != "<@!1234>hello world" {
		t.Errorf("content = %v", body["content"])
	}
	if resp.ID != "c1" || resp.ChannelID != "123" || resp.GuildID != "456" {
		t.Errorf("resp = %+v", resp)
	}
	if resp.Author == nil || resp.Author.Username != "abc" || !resp.Author.Bot {
		t.Errorf("Author = %+v, want the decoded author", resp.Author)
	}
}

// TestSendChannelMessageWithEmbed covers the embed card body.
func TestSendChannelMessageWithEmbed(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"c1"}`)

	_, err := client.SendChannelMessage(t.Context(), "123", &ChannelMessage{
		Embed: &MessageEmbed{
			Title:     "标题",
			Prompt:    "消息通知",
			Thumbnail: &MessageEmbedThumbnail{URL: "https://example.com/a.png"},
			Fields:    []MessageEmbedField{{Name: "当前等级：黄金"}, {Name: "之前等级：白银"}},
		},
	})
	if err != nil {
		t.Fatalf("SendChannelMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	embed := body["embed"].(map[string]any)
	if embed["title"] != "标题" || embed["prompt"] != "消息通知" {
		t.Errorf("embed = %v", embed)
	}
	fields := embed["fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("fields = %v, want 2 entries", fields)
	}
	if fields[0].(map[string]any)["name"] != "当前等级：黄金" {
		t.Errorf("first field = %v", fields[0])
	}
	if embed["thumbnail"].(map[string]any)["url"] != "https://example.com/a.png" {
		t.Errorf("thumbnail = %v", embed["thumbnail"])
	}
}

// TestSendChannelMessageWithArk covers the structured card body, including the
// nested array variable form.
func TestSendChannelMessageWithArk(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"c1"}`)

	_, err := client.SendChannelMessage(t.Context(), "123", &ChannelMessage{
		Ark: &MessageArk{
			TemplateID: 23,
			KV: []MessageArkKV{
				{Key: "#DESC#", Value: "机器人订阅消息"},
				{Key: "#PROMPT#", Value: "XX机器人"},
				{Key: "#LIST#", Obj: []MessageArkObj{
					{ObjKV: []MessageArkObjKV{{Key: "desc", Value: "文本"}}},
					{ObjKV: []MessageArkObjKV{{Key: "desc", Value: "已评审"}, {Key: "link", Value: "https://qun.qq.com"}}},
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendChannelMessage: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	ark := body["ark"].(map[string]any)
	if ark["template_id"] != float64(23) {
		t.Errorf("template_id = %v, want 23", ark["template_id"])
	}
	kv := ark["kv"].([]any)
	if len(kv) != 3 {
		t.Fatalf("kv = %v, want 3 entries", kv)
	}
	list := kv[2].(map[string]any)
	obj := list["obj"].([]any)
	if len(obj) != 2 {
		t.Fatalf("obj = %v, want 2 entries", obj)
	}
	second := obj[1].(map[string]any)["obj_kv"].([]any)
	if second[1].(map[string]any)["value"] != "https://qun.qq.com" {
		t.Errorf("nested obj_kv = %v", second)
	}
}

func TestSendDirectMessage(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"d1","channel_id":"ch","guild_id":"g"}`)

	if _, err := client.SendDirectMessage(t.Context(), "GUILD1", &ChannelMessage{Content: "hi"}); err != nil {
		t.Fatalf("SendDirectMessage: %v", err)
	}
	if got := last(t, captured).Path; got != "/dms/GUILD1/messages" {
		t.Errorf("path = %s, want /dms/GUILD1/messages", got)
	}
}

func TestCreateDirectMessageSession(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK,
		`{"guild_id":"xxxxxx","channel_id":"yyyyyy","create_time":"1642545606"}`)

	dms, err := client.CreateDirectMessageSession(t.Context(), "123456", "112233")
	if err != nil {
		t.Fatalf("CreateDirectMessageSession: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/users/@me/dms" {
		t.Errorf("path = %s, want /users/@me/dms", req.Path)
	}
	body := decodeBody(t, req)
	if body["recipient_id"] != "123456" || body["source_guild_id"] != "112233" {
		t.Errorf("body = %v", body)
	}
	if dms.GuildID != "xxxxxx" || dms.ChannelID != "yyyyyy" || dms.CreateTime != "1642545606" {
		t.Errorf("dms = %+v", dms)
	}
}

// TestChatSendRejectsMissingIDs covers the local guards, which save a pointless
// round trip.
func TestChatSendRejectsMissingIDs(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	calls := map[string]func() error{
		"SendC2CMessage":       func() error { _, err := client.SendC2CMessage(t.Context(), "", &Message{}); return err },
		"SendGroupMessage":     func() error { _, err := client.SendGroupMessage(t.Context(), "", &Message{}); return err },
		"SendC2CStreamMessage": func() error { _, err := client.SendC2CStreamMessage(t.Context(), "", &StreamMessage{}); return err },
		"SendChannelMessage":   func() error { _, err := client.SendChannelMessage(t.Context(), "", &ChannelMessage{}); return err },
		"SendChannelMessageMultipart": func() error {
			_, err := client.SendChannelMessageMultipart(t.Context(), "", &ChannelMessage{}, nil)
			return err
		},
		"SendDirectMessage":          func() error { _, err := client.SendDirectMessage(t.Context(), "", &ChannelMessage{}); return err },
		"CreateDirectMessageSession": func() error { _, err := client.CreateDirectMessageSession(t.Context(), "a", ""); return err },
		"UploadC2CFile":              func() error { _, err := client.UploadC2CFile(t.Context(), "", nil); return err },
		"UploadGroupFile":            func() error { _, err := client.UploadGroupFile(t.Context(), "", nil); return err },
		"PrepareC2CUpload":           func() error { _, err := client.PrepareC2CUpload(t.Context(), "", nil); return err },
		"PrepareGroupUpload":         func() error { _, err := client.PrepareGroupUpload(t.Context(), "", nil); return err },
		"FinishC2CUploadPart":        func() error { return client.FinishC2CUploadPart(t.Context(), "", nil) },
		"FinishGroupUploadPart":      func() error { return client.FinishGroupUploadPart(t.Context(), "", nil) },
		"RecallC2CMessage":           func() error { return client.RecallC2CMessage(t.Context(), "", "m") },
		"RecallGroupMessage":         func() error { return client.RecallGroupMessage(t.Context(), "", "m") },
		"RecallChannelMessage":       func() error { return client.RecallChannelMessage(t.Context(), "", "m", false) },
		"RecallDirectMessage":        func() error { return client.RecallDirectMessage(t.Context(), "", "m", false) },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("a missing id must be rejected before any request")
			}
		})
	}

	if len(*captured) != 0 {
		t.Errorf("a rejected call still sent %d requests", len(*captured))
	}
}

// TestRecallEndpoints checks the documented delete paths and query parameter.
func TestRecallEndpoints(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.RecallC2CMessage(t.Context(), "USER1", "MSG1"); err != nil {
		t.Fatalf("RecallC2CMessage: %v", err)
	}
	if err := client.RecallGroupMessage(t.Context(), "GROUP1", "MSG2"); err != nil {
		t.Fatalf("RecallGroupMessage: %v", err)
	}
	if err := client.RecallChannelMessage(t.Context(), "CH1", "MSG3", true); err != nil {
		t.Fatalf("RecallChannelMessage: %v", err)
	}
	if err := client.RecallDirectMessage(t.Context(), "GUILD1", "MSG4", false); err != nil {
		t.Fatalf("RecallDirectMessage: %v", err)
	}

	requests := *captured
	if len(requests) != 4 {
		t.Fatalf("sent %d requests, want 4", len(requests))
	}
	wantPaths := []string{
		"/v2/users/USER1/messages/MSG1",
		"/v2/groups/GROUP1/messages/MSG2",
		"/channels/CH1/messages/MSG3",
		"/dms/GUILD1/messages/MSG4",
	}
	for i, want := range wantPaths {
		if requests[i].Path != want {
			t.Errorf("request %d path = %s, want %s", i, requests[i].Path, want)
		}
		if requests[i].Method != http.MethodDelete {
			t.Errorf("request %d method = %s, want DELETE", i, requests[i].Method)
		}
	}
	if requests[2].Query != "hidetip=true" {
		t.Errorf("channel query = %q, want hidetip=true", requests[2].Query)
	}
	if requests[3].Query != "hidetip=false" {
		t.Errorf("dms query = %q, want hidetip=false", requests[3].Query)
	}
}

// TestSendChannelMessageMultipart checks the multipart body the endpoint accepts.
func TestSendChannelMessageMultipart(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"c1"}`)

	_, err := client.SendChannelMessageMultipart(t.Context(), "123", &ChannelMessage{
		Content: "<@!1234>hello world",
		Ark:     &MessageArk{TemplateID: 1, KV: []MessageArkKV{{Key: "#DESC#", Value: "订阅消息"}}},
		MsgID:   "xxxxxx",
	}, &ChannelMessageFile{FileName: "pic.png", Content: bytes.NewReader([]byte("PNGDATA"))})
	if err != nil {
		t.Fatalf("SendChannelMessageMultipart: %v", err)
	}

	req := last(t, captured)
	if !strings.HasPrefix(req.ContentType, "multipart/form-data") {
		t.Fatalf("Content-Type = %q, want multipart/form-data", req.ContentType)
	}
	body := string(req.Body)
	for _, want := range []string{
		`name="content"`, "<@!1234>hello world",
		`name="ark"`, `{"template_id":1,`, // object fields are JSON strings
		`name="msg_id"`, "xxxxxx",
		`name="file_image"`, "pic.png", "PNGDATA",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("multipart body is missing %q", want)
		}
	}
}

// TestSendChannelMessageMultipartWithoutFile checks the attachment-free path.
func TestSendChannelMessageMultipartWithoutFile(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"c1"}`)

	if _, err := client.SendChannelMessageMultipart(t.Context(), "123", &ChannelMessage{Content: "hi"}, nil); err != nil {
		t.Fatalf("SendChannelMessageMultipart: %v", err)
	}
	body := string(last(t, captured).Body)
	if strings.Contains(body, "file_image") {
		t.Error("no file was passed, so no file part should be written")
	}
	if !strings.Contains(body, `name="content"`) {
		t.Error("the content field must still be written")
	}
}

// TestMessageErrorIsOpenAPIError checks that a documented business failure is
// reported with its code and trace id.
func TestMessageErrorIsOpenAPIError(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"err_code":40034005,"message":"回复消息msg_id已过期","trace_id":"trace-1"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{Content: "x", MsgID: "expired"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	openAPIErr, ok := err.(*OpenAPIError)
	if !ok {
		t.Fatalf("err = %T, want *OpenAPIError", err)
	}
	if openAPIErr.Code != 40034005 {
		t.Errorf("Code = %d, want 40034005", openAPIErr.Code)
	}
	if openAPIErr.TraceID != "trace-1" {
		t.Errorf("TraceID = %q", openAPIErr.TraceID)
	}
	if !strings.Contains(openAPIErr.URL, "/v2/users/USER1/messages") {
		t.Errorf("URL = %q, want the request address", openAPIErr.URL)
	}
}

// TestMessagePathsEscapeIDs checks that ids are path escaped, so an id cannot
// add path segments or change the route.
func TestMessagePathsEscapeIDs(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"m"}`)

	if _, err := client.SendC2CMessage(t.Context(), "a/b c", &Message{Content: "x"}); err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}

	req := last(t, captured)
	if !strings.Contains(req.EscapedPath, "%2F") {
		t.Errorf("escaped path = %s, want the slash encoded", req.EscapedPath)
	}
	if strings.Contains(req.EscapedPath, " ") {
		t.Errorf("escaped path = %s, want the space encoded", req.EscapedPath)
	}
}

// TestSendEndpointsPropagateErrors checks that every send method surfaces a
// platform failure instead of returning an empty response.
func TestSendEndpointsPropagateErrors(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":304061,"message":"消息内容无效"}`)

	calls := map[string]func() error{
		"SendC2CMessage":   func() error { _, err := client.SendC2CMessage(t.Context(), "U", &Message{Content: "x"}); return err },
		"SendGroupMessage": func() error { _, err := client.SendGroupMessage(t.Context(), "G", &Message{Content: "x"}); return err },
		"SendC2CStreamMessage": func() error {
			_, err := client.SendC2CStreamMessage(t.Context(), "U", &StreamMessage{})
			return err
		},
		"SendChannelMessage": func() error {
			_, err := client.SendChannelMessage(t.Context(), "C", &ChannelMessage{Content: "x"})
			return err
		},
		"SendChannelMessageMultipart": func() error {
			_, err := client.SendChannelMessageMultipart(t.Context(), "C", &ChannelMessage{Content: "x"}, nil)
			return err
		},
		"SendDirectMessage": func() error {
			_, err := client.SendDirectMessage(t.Context(), "G", &ChannelMessage{Content: "x"})
			return err
		},
		"CreateDirectMessageSession": func() error {
			_, err := client.CreateDirectMessageSession(t.Context(), "r", "g")
			return err
		},
		"UploadC2CFile":    func() error { _, err := client.UploadC2CFile(t.Context(), "U", &FileUploadRequest{}); return err },
		"UploadGroupFile":  func() error { _, err := client.UploadGroupFile(t.Context(), "G", &FileUploadRequest{}); return err },
		"PrepareC2CUpload": func() error { _, err := client.PrepareC2CUpload(t.Context(), "U", &UploadPrepareRequest{}); return err },
		"PrepareGroupUpload": func() error {
			_, err := client.PrepareGroupUpload(t.Context(), "G", &UploadPrepareRequest{})
			return err
		},
		"FinishC2CUploadPart": func() error { return client.FinishC2CUploadPart(t.Context(), "U", &UploadPartFinishRequest{}) },
		"FinishGroupUploadPart": func() error {
			return client.FinishGroupUploadPart(t.Context(), "G", &UploadPartFinishRequest{})
		},
		"RecallC2CMessage":     func() error { return client.RecallC2CMessage(t.Context(), "U", "m") },
		"RecallGroupMessage":   func() error { return client.RecallGroupMessage(t.Context(), "G", "m") },
		"RecallChannelMessage": func() error { return client.RecallChannelMessage(t.Context(), "C", "m", false) },
		"RecallDirectMessage":  func() error { return client.RecallDirectMessage(t.Context(), "G", "m", false) },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("a platform failure must be reported, not swallowed")
			}
		})
	}
}

// TestMessageErrorCodesAreNamed checks that the per-endpoint message codes
// resolve to their documented text, so an error renders usefully.
func TestMessageErrorCodesAreNamed(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrMsgTypeMismatch:              "消息类型与内容不匹配",
		ErrReplyMsgIDExpired:            "回复消息msg_id已过期",
		ErrMessageDeduplicated:          "消息被去重",
		ErrFileTooLarge:                 "上传文件超过大小限制",
		ErrRecallTimeExceeded:           "已超出消息撤回时限",
		ErrStreamPrefixImmutable:        "已下发内容前缀不可修改",
		ErrMarkdownTemplateNoPermission: "无markdown模板权限",
		ErrBotMuted:                     "机器人被禁言",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpenAPIErrorCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}

// TestMessageErrorRendering checks that a message failure names its code text.
func TestMessageErrorRendering(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"err_code":40054005,"message":"消息被去重"}`)

	_, err := client.SendC2CMessage(t.Context(), "USER1", &Message{Content: "x"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "消息被去重") {
		t.Errorf("err = %v, want the documented description", err)
	}
	if !IsOpenAPIError(err, ErrMessageDeduplicated) {
		t.Errorf("err = %v, want the deduplication code to match", err)
	}
}
