//go:build production

package qqbotsdk_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionInteractionResponse answers a real button click.
//
// It needs a person: OneBot cannot click an inline keyboard button, so the test
// sends the button message, waits for the click and answers it. Nothing runs
// unless QQBOT_INTERACTION_TEST is set, so an ordinary production run never
// sits waiting for a human.
//
// The second half checks the documented rule that an interaction may be
// answered only once, which is exactly why the SDK does not answer on its own.
func TestProductionInteractionResponse(t *testing.T) {
	if os.Getenv("QQBOT_INTERACTION_TEST") == "" {
		t.Skip("set QQBOT_INTERACTION_TEST=1, run this test, and click the button on the test account")
	}

	cfg := loadProductionConfig(t)

	// The interaction category rides on the same connection as the single chat
	// events the harness already needs.
	client, received := c2cHarness(t, cfg, qqbotsdk.IntentInteraction)

	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
	defer cancel()

	interactions := make(chan *qqbotsdk.InteractionCreateData, 8)
	client.RegisterFunc(qqbotsdk.EventInteractionCreate, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			t.Errorf("decoding INTERACTION_CREATE: %v", err)
			return err
		}
		data, ok := value.(*qqbotsdk.InteractionCreateData)
		if !ok {
			t.Errorf("INTERACTION_CREATE decoded to %T", value)
			return nil
		}
		t.Logf("event INTERACTION_CREATE: id=%s type=%d scene=%s chat_type=%d needs_response=%v",
			data.ID, data.Type, data.Scene, data.ChatType, data.NeedsResponse())
		if data.Data != nil && data.Data.Resolved != nil {
			resolved := data.Data.Resolved
			t.Logf("  resolved: button_id=%q button_data=%q message_id=%q",
				resolved.ButtonID, resolved.ButtonData, resolved.MessageID)
		}
		select {
		case interactions <- data:
		default:
		}
		return nil
	})

	marker := fmt.Sprintf("CLICK-%d", time.Now().UnixNano())
	if _, err := client.SendC2CMessage(ctx, received.Author.UserOpenID, &qqbotsdk.Message{
		MsgType:  qqbotsdk.MsgTypeMarkdown,
		Markdown: &qqbotsdk.MessageMarkdown{Content: "**" + marker + "**\n请点击下方按钮，机器人会回应这次互动："},
		Keyboard: &qqbotsdk.Keyboard{Content: &qqbotsdk.KeyboardContent{Rows: []qqbotsdk.Row{{
			Buttons: []qqbotsdk.Button{{
				ID: "btn_interaction",
				RenderData: &qqbotsdk.RenderData{
					Label:        "回调按钮",
					VisitedLabel: "已点击",
					Style:        qqbotsdk.KeyboardStyleBlue,
				},
				Action: &qqbotsdk.Action{
					Type:          qqbotsdk.ActionTypeCallback,
					Data:          "interaction-production-test",
					Permission:    &qqbotsdk.Permission{Type: qqbotsdk.PermissionTypeEveryone},
					UnsupportTips: "请升级 QQ 客户端",
				},
			}},
		}}}},
		MsgID:  received.ID,
		MsgSeq: 1,
	}); err != nil {
		t.Fatalf("sending the button message: %v", err)
	}

	t.Logf("sent the button message %s — click 「回调按钮」 on the test account now", marker)

	var interaction *qqbotsdk.InteractionCreateData
	deadline := time.After(5 * time.Minute)
	for interaction == nil {
		select {
		case data := <-interactions:
			if data.Type == qqbotsdk.InteractionInlineKeyboard {
				interaction = data
			} else {
				t.Logf("ignoring an interaction of type %d", data.Type)
			}
		case <-deadline:
			t.Fatal("no button click arrived within five minutes")
		}
	}

	if interaction.ID == "" {
		t.Fatal("the interaction carries no id to answer")
	}
	if !interaction.NeedsResponse() {
		t.Errorf("type %d should not need a response", interaction.Type)
	}

	started := time.Now()
	if err := client.RespondInteraction(ctx, interaction.ID, qqbotsdk.InteractionCodeSuccess); err != nil {
		t.Fatalf("RespondInteraction: %v", err)
	}
	t.Logf("answered the interaction in %s; the client should have stopped loading",
		time.Since(started).Round(time.Millisecond))

	// An interaction may be answered once, so this second call must be refused.
	err := client.RespondInteraction(ctx, interaction.ID, qqbotsdk.InteractionCodeSuccess)
	if err == nil {
		t.Error("answering the same interaction twice must be refused")
	} else {
		t.Logf("the second answer was refused as documented: %v", err)
	}

	// The prefix must never be sent, so the answered id is the bare one, which
	// is what makes the call above work at all.
	if err := client.RespondInteraction(ctx, "", qqbotsdk.InteractionCodeSuccess); err == nil {
		t.Error("an empty interaction id must be refused locally")
	}
}
