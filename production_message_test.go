//go:build production

package qqbotsdk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// c2cHarness connects to the gateway and waits for one private message the
// controlled account sends the bot, returning the client and that event.
//
// Each caller gets a fresh user message, so every test starts with its own
// passive reply budget: the documentation allows four replies per single chat
// message, and a shared trigger would exhaust it.
func c2cHarness(t *testing.T, cfg productionConfig) (*qqbotsdk.Client, *qqbotsdk.C2CMessageCreateData) {
	t.Helper()

	client := productionClient(cfg)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)

	gatewayURL, err := cachedGateway(ctx, client)
	if err != nil {
		cancel()
		t.Fatalf("GetGateway: %v", err)
	}

	ready := make(chan struct{}, 1)
	messages := make(chan *qqbotsdk.C2CMessageCreateData, 8)

	client.RegisterFunc(qqbotsdk.EventReady, func(_ context.Context, _ *qqbotsdk.Event) error {
		select {
		case ready <- struct{}{}:
		default:
		}
		return nil
	})
	client.RegisterFunc(qqbotsdk.EventC2CMessageCreate, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			return err
		}
		select {
		case messages <- value.(*qqbotsdk.C2CMessageCreateData):
		default:
		}
		return nil
	})

	client.UseTransport(qqbotsdk.NewWebSocketTransport(gatewayURL,
		qqbotsdk.WithIntents(qqbotsdk.IntentGroupAndC2CEvent)))
	if err := client.Start(ctx); err != nil {
		cancel()
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})

	select {
	case <-ready:
	case <-time.After(45 * time.Second):
		t.Fatal("the websocket never became ready")
	}

	probe := fmt.Sprintf("TYPES-%d", time.Now().UnixNano())
	oneBotSendPrivate(t, cfg, cfg.botQQ, probe)
	t.Logf("the test account sent the probe %q", probe)

	deadline := time.After(90 * time.Second)
	for {
		select {
		case data := <-messages:
			if strings.Contains(data.Content, probe) {
				return client, data
			}
		case <-deadline:
			t.Fatal("the probe never reached the bot")
		}
	}
}

// TestProductionC2CMarkdownAndKeyboard sends a markdown reply and a keyboard
// reply to the same user message, which exercises both request structures.
func TestProductionC2CMarkdownAndKeyboard(t *testing.T) {
	cfg := loadProductionConfig(t)
	client, received := c2cHarness(t, cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	marker := fmt.Sprintf("MD-%d", time.Now().UnixNano())
	markdown := &qqbotsdk.Message{
		MsgType: qqbotsdk.MsgTypeMarkdown,
		Markdown: &qqbotsdk.MessageMarkdown{
			Content: "**" + marker + "**\n> markdown 生产测试",
		},
		MsgID:  received.ID,
		MsgSeq: 1,
	}
	response, err := client.SendC2CMessage(ctx, received.Author.UserOpenID, markdown)
	if err != nil {
		t.Fatalf("markdown reply: %v", err)
	}
	t.Logf("markdown accepted: id=%s timestamp=%s", response.ID, response.Timestamp)

	// A keyboard attaches to a markdown message: the documentation opens with
	// "在 markdown 消息的基础上，支持消息最底部挂载按钮". Sending it with a plain
	// text message is accepted but the buttons are dropped silently, so the
	// message must be markdown for the keyboard to render at all.
	keyboardMarker := fmt.Sprintf("BTN-%d", time.Now().UnixNano())
	keyboard := &qqbotsdk.Message{
		MsgType: qqbotsdk.MsgTypeMarkdown,
		Markdown: &qqbotsdk.MessageMarkdown{
			Content: "**" + keyboardMarker + "**\n请在下方选择：",
		},
		Keyboard: &qqbotsdk.Keyboard{Content: &qqbotsdk.KeyboardContent{
			Rows: []qqbotsdk.Row{{Buttons: []qqbotsdk.Button{{
				ID: "btn-1",
				RenderData: &qqbotsdk.RenderData{
					Label:        "回调按钮",
					VisitedLabel: "已点击",
					Style:        qqbotsdk.KeyboardStyleBlue,
				},
				Action: &qqbotsdk.Action{
					Type:       qqbotsdk.ActionTypeCallback,
					Data:       "ping",
					Permission: &qqbotsdk.Permission{Type: qqbotsdk.PermissionTypeEveryone},
					// The field table marks this required.
					UnsupportTips: "请升级 QQ 客户端",
				},
			}}}},
		}},
		MsgID:  received.ID,
		MsgSeq: 2,
	}
	response, err = client.SendC2CMessage(ctx, received.Author.UserOpenID, keyboard)
	if err != nil {
		t.Fatalf("keyboard reply: %v", err)
	}
	t.Logf("keyboard accepted: id=%s timestamp=%s", response.ID, response.Timestamp)

	oneBotVerifyDelivered(t, cfg, "get_friend_msg_history",
		map[string]any{"user_id": jsonNumber(cfg.botQQ), "count": 20},
		marker, "the controlled account")
}

// TestProductionC2CMedia uploads a real image and sends it back as a rich media
// reply, which exercises the upload endpoint and the media message together.
func TestProductionC2CMedia(t *testing.T) {
	cfg := loadProductionConfig(t)
	client, received := c2cHarness(t, cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	// A stable, publicly reachable image, so the platform can fetch it.
	const imageURL = "https://q.qlogo.cn/headimg_dl?dst_uin=10000&spec=640"
	uploaded, err := client.UploadC2CFile(ctx, received.Author.UserOpenID, &qqbotsdk.FileUploadRequest{
		FileType: qqbotsdk.FileTypeImage,
		URL:      imageURL,
	})
	if err != nil {
		t.Fatalf("UploadC2CFile: %v", err)
	}
	if uploaded.FileInfo == "" {
		t.Fatalf("the upload returned no file_info: %+v", uploaded)
	}
	t.Logf("uploaded: file_uuid=%s ttl=%d file_info=%d chars",
		uploaded.FileUUID, uploaded.TTL, len(uploaded.FileInfo))

	response, err := client.SendC2CMessage(ctx, received.Author.UserOpenID, &qqbotsdk.Message{
		MsgType: qqbotsdk.MsgTypeMedia,
		Media:   &qqbotsdk.MediaInfo{FileInfo: uploaded.FileInfo},
		MsgID:   received.ID,
		MsgSeq:  1,
	})
	if err != nil {
		t.Fatalf("media reply: %v", err)
	}
	t.Logf("media reply accepted: id=%s timestamp=%s", response.ID, response.Timestamp)
}

// TestProductionC2CStream sends a streaming reply in three chunks, following
// the documented pattern exactly.
//
// Every documented example uses input_mode replace with cumulative content:
// each chunk repeats the whole text so far, and the platform requires the new
// body to start with the prefix it already sent. Sending only the delta with
// append is answered with "已经提交的消息内容不可修改" (40113003), so the
// documented shape is the one exercised here.
func TestProductionC2CStream(t *testing.T) {
	cfg := loadProductionConfig(t)
	client, received := c2cHarness(t, cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	chunks := []struct {
		index int
		state int
		text  string
	}{
		{0, qqbotsdk.StreamInputGenerating, "流式生产测试"},
		{1, qqbotsdk.StreamInputGenerating, "流式生产测试：第二片已到达"},
		{2, qqbotsdk.StreamInputFinished, "流式生产测试：第二片已到达。生成结束。"},
	}

	var streamID string
	for i, chunk := range chunks {
		message := &qqbotsdk.StreamMessage{
			InputMode:   qqbotsdk.StreamInputReplace,
			InputState:  chunk.state,
			Index:       chunk.index,
			ContentType: qqbotsdk.StreamContentText,
			ContentRaw:  chunk.text,
			MsgID:       received.ID,
			MsgSeq:      1,
			StreamMsgID: streamID, // empty on the first chunk
		}
		response, err := client.SendC2CStreamMessage(ctx, received.Author.UserOpenID, message)
		if err != nil {
			t.Fatalf("stream chunk %d (index %d): %v", i, chunk.index, err)
		}
		if i == 0 {
			streamID = response.StreamID()
			if streamID == "" {
				t.Fatal("the first chunk returned no id to continue the stream with")
			}
		}
		t.Logf("chunk index=%d state=%d accepted: id=%s remain=%d",
			chunk.index, chunk.state, response.ID, response.RemainMsgLen)
	}

	// The platform accepted every chunk, but the receiving QQ client does not
	// render a stream message: the conversation shows the placeholder
	// "[暂不支持该消息类型，请用最新版手机 QQ 查看]" instead of the text.
	// That placeholder is therefore the observable evidence of delivery, and
	// matching the streamed text would only ever fail.
	oneBotVerifyDelivered(t, cfg, "get_friend_msg_history",
		map[string]any{"user_id": jsonNumber(cfg.botQQ), "count": 20},
		"暂不支持该消息类型", "the controlled account (as an unrenderable message)")
}

// jsonNumber keeps the OneBot payloads numeric, since that client rejects a
// numeric id sent as a string.
func jsonNumber(id string) json.Number { return json.Number(id) }
