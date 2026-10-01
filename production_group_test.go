//go:build production

package qqbotsdk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionGroupRoundTrip mentions the bot in a real group through OneBot,
// receives the resulting GROUP_AT_MESSAGE_CREATE on the gateway websocket, and
// answers it with a passive group message.
//
// The group is read from QQBOT_TEST_GROUP, so no live group id is baked into
// this file. Only the mention triggers GROUP_AT_MESSAGE_CREATE; a group in full
// receive mode would also send GROUP_MESSAGE_CREATE, so both are watched.
func TestProductionGroupRoundTrip(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupID := os.Getenv("QQBOT_TEST_GROUP")
	if groupID == "" {
		t.Skip("set QQBOT_TEST_GROUP to the group the bot shares with the test account")
	}
	if _, err := strconv.ParseInt(groupID, 10, 64); err != nil {
		t.Fatalf("QQBOT_TEST_GROUP must be a number, got %q", groupID)
	}

	client := productionClient(cfg)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	gatewayURL, err := cachedGateway(ctx, client)
	if err != nil {
		t.Fatalf("GetGateway: %v", err)
	}

	type groupEvent struct {
		eventType string
		data      *qqbotsdk.GroupMessageCreateData
	}
	ready := make(chan struct{}, 1)
	events := make(chan groupEvent, 16)

	client.RegisterFunc(qqbotsdk.EventReady, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			return err
		}
		t.Logf("READY: session=%s", value.(*qqbotsdk.ReadyData).SessionID)
		select {
		case ready <- struct{}{}:
		default:
		}
		return nil
	})

	collect := func(eventType string) func(context.Context, *qqbotsdk.Event) error {
		return func(_ context.Context, event *qqbotsdk.Event) error {
			value, err := event.Decode()
			if err != nil {
				t.Errorf("decoding %s: %v", eventType, err)
				return err
			}
			data, ok := value.(*qqbotsdk.GroupMessageCreateData)
			if !ok {
				t.Errorf("%s decoded to %T", eventType, value)
				return nil
			}
			t.Logf("event %s: id=%s group=%s content=%q type=%d author=%s",
				eventType, data.ID, data.GroupOpenID, data.Content, data.MessageType,
				data.Author.MemberOpenID)
			select {
			case events <- groupEvent{eventType: eventType, data: data}:
			default:
			}
			return nil
		}
	}
	client.RegisterFunc(qqbotsdk.EventGroupAtMessageCreate, collect(qqbotsdk.EventGroupAtMessageCreate))
	client.RegisterFunc(qqbotsdk.EventGroupMessageCreate, collect(qqbotsdk.EventGroupMessageCreate))

	client.UseTransport(qqbotsdk.NewWebSocketTransport(gatewayURL,
		qqbotsdk.WithIntents(qqbotsdk.IntentGroupAndC2CEvent)))
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
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

	probe := fmt.Sprintf("GROUP-PING-%d", time.Now().UnixNano())
	oneBotMention(t, cfg, groupID, probe)
	t.Logf("the test account mentioned the bot in group %s with %q", groupID, probe)

	var received groupEvent
	deadline := time.After(120 * time.Second)
	for received.data == nil {
		select {
		case incoming := <-events:
			if strings.Contains(incoming.data.Content, probe) {
				received = incoming
			} else {
				// Another member, or the group's own bot, talking.
				t.Logf("ignoring another %s message: %q", incoming.eventType, incoming.data.Content)
			}
		case <-deadline:
			t.Fatal("the bot never received the group probe")
		}
	}

	if received.data.GroupOpenID == "" {
		t.Fatal("the group event carries no group openid")
	}
	if received.data.Author == nil || received.data.Author.MemberOpenID == "" {
		t.Fatalf("the group event carries no member openid: %+v", received.data.Author)
	}
	t.Logf("author role in the group: %q", received.data.Author.MemberRole)

	// A group passive reply carries the event body's message id, valid for the
	// documented five minutes.
	replyText := "GROUP-PONG " + probe
	response, err := client.SendGroupMessage(ctx, received.data.GroupOpenID, &qqbotsdk.Message{
		Content: replyText,
		MsgID:   received.data.ID,
		MsgSeq:  1,
	})
	if err != nil {
		t.Fatalf("SendGroupMessage: %v", err)
	}
	if response.ID == "" {
		t.Error("the platform accepted the group reply but returned no message id")
	}
	t.Logf("group reply accepted: id=%s timestamp=%s", response.ID, response.Timestamp)

	oneBotVerifyDelivered(t, cfg, "get_group_msg_history",
		map[string]any{"group_id": json.Number(groupID), "count": 20},
		replyText, "the group")

	// A second passive reply to the same message exercises markdown in a
	// group; the documentation allows five replies to one group message.
	markdownText := "GROUP-MD " + probe
	if _, err := client.SendGroupMessage(ctx, received.data.GroupOpenID, &qqbotsdk.Message{
		MsgType:  qqbotsdk.MsgTypeMarkdown,
		Markdown: &qqbotsdk.MessageMarkdown{Content: "**" + markdownText + "**"},
		MsgID:    received.data.ID,
		MsgSeq:   2,
	}); err != nil {
		t.Fatalf("group markdown reply: %v", err)
	}
	t.Log("group markdown reply accepted")

	oneBotVerifyDelivered(t, cfg, "get_group_msg_history",
		map[string]any{"group_id": json.Number(groupID), "count": 20},
		markdownText, "the group")
}

// oneBotMention sends a group message that mentions the bot, which is what
// triggers GROUP_AT_MESSAGE_CREATE.
func oneBotMention(t *testing.T, cfg productionConfig, groupID, text string) int64 {
	t.Helper()
	data := oneBotCall(t, cfg, "send_group_msg", map[string]any{
		"group_id": json.Number(groupID),
		"message": []map[string]any{
			{"type": "at", "data": map[string]any{"qq": cfg.botQQ}},
			{"type": "text", "data": map[string]any{"text": " " + text}},
		},
	})
	id, _ := data["message_id"].(float64)
	if id == 0 {
		t.Fatalf("OneBot did not report a group message id: %v", data)
	}
	return int64(id)
}
