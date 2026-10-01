//go:build production

package qqbotsdk_test

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionInteractionBehaviours drives every interactive branch that
// needs a person, in one sitting, because a button click cannot be automated.
//
// Three messages go out, and the test then answers whatever arrives according to
// each button's data, logging every click with its time. Which clicks arrive is
// itself the measurement:
//
//   - A1, A2, A3 are three callback buttons with no group, so clicking one
//     should not touch the others. A1 and A3 are answered with success, A2 with
//     failure, which is the open question: does a failed answer leave the button
//     clickable? Clicking A2 twice answers it.
//   - B1 and B2 share a group id. The send documentation says that once one
//     button of a group is used, the others grey out, so B2 should not click at
//     all.
//   - C1 is a command button, which the documentation says inserts text into
//     the composer rather than reporting an interaction, so no event is
//     expected.
func TestProductionInteractionBehaviours(t *testing.T) {
	if os.Getenv("QQBOT_INTERACTION_TEST") == "" {
		t.Skip("set QQBOT_INTERACTION_TEST=1, run this test, and click the buttons on the test account")
	}

	cfg := loadProductionConfig(t)
	client, received := c2cHarness(t, cfg, qqbotsdk.IntentInteraction)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()

	// The answer each button gets, which is what makes the run reproducible.
	answers := map[string]qqbotsdk.InteractionCode{
		"a1": qqbotsdk.InteractionCodeSuccess,
		"a2": qqbotsdk.InteractionCodeFailed, // the question: does it stay clickable?
		"a3": qqbotsdk.InteractionCodeSuccess,
		"b1": qqbotsdk.InteractionCodeSuccess,
		"b2": qqbotsdk.InteractionCodeSuccess,
	}

	var (
		mu     sync.Mutex
		clicks []string
		start  = time.Now()
	)
	client.RegisterFunc(qqbotsdk.EventInteractionCreate, func(handlerCtx context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			t.Errorf("decoding INTERACTION_CREATE: %v", err)
			return err
		}
		data := value.(*qqbotsdk.InteractionCreateData)
		button := ""
		if data.Data != nil && data.Data.Resolved != nil {
			button = data.Data.Resolved.ButtonData
		}

		elapsed := time.Since(start).Round(time.Millisecond)
		mu.Lock()
		clicks = append(clicks, button)
		mu.Unlock()
		t.Logf("click #%d at +%s: id=%s type=%d button_data=%q",
			len(clicks), elapsed, data.ID, data.Type, button)

		if !data.NeedsResponse() {
			t.Logf("  %s does not need an answer", button)
			return nil
		}
		code, known := answers[button]
		if !known {
			code = qqbotsdk.InteractionCodeSuccess
		}
		if err := client.RespondInteraction(handlerCtx, data.ID, code); err != nil {
			t.Errorf("answering %s with code %d: %v", button, code, err)
			return err
		}
		t.Logf("  answered %s with %d (%s)", button, code, code)
		return nil
	})

	// Three messages, each a passive reply to the same probe, so all three fit
	// in the four answers a single chat message allows.
	send := func(seq int, text string, rows []qqbotsdk.Row) {
		t.Helper()
		if _, err := client.SendC2CMessage(ctx, received.Author.UserOpenID, &qqbotsdk.Message{
			MsgType:  qqbotsdk.MsgTypeMarkdown,
			Markdown: &qqbotsdk.MessageMarkdown{Content: text},
			Keyboard: &qqbotsdk.Keyboard{Content: &qqbotsdk.KeyboardContent{Rows: rows}},
			MsgID:    received.ID,
			MsgSeq:   seq,
		}); err != nil {
			t.Fatalf("sending message %d: %v", seq, err)
		}
	}

	send(1, "**【A】独立按钮**\n请依次点：A1 → A2 → **再点一次 A2** → A3", []qqbotsdk.Row{{
		Buttons: []qqbotsdk.Button{
			callbackButton("btn_a1", "A1", "a1", ""),
			callbackButton("btn_a2", "A2", "a2", ""),
			callbackButton("btn_a3", "A3", "a3", ""),
		},
	}})

	send(2, "**【B】同组按钮**\n先点 B1，然后**试着点 B2**（预期：点不了）", []qqbotsdk.Row{{
		Buttons: []qqbotsdk.Button{
			callbackButton("btn_b1", "B1", "b1", "group_b"),
			callbackButton("btn_b2", "B2", "b2", "group_b"),
		},
	}})

	send(3, "**【C】指令按钮**\n点 C1，然后看输入框里出现了什么", []qqbotsdk.Row{{
		Buttons: []qqbotsdk.Button{{
			ID: "btn_c1",
			RenderData: &qqbotsdk.RenderData{
				Label: "C1", VisitedLabel: "C1", Style: qqbotsdk.KeyboardStyleGrey,
			},
			Action: &qqbotsdk.Action{
				Type:          qqbotsdk.ActionTypeCommand,
				Data:          "/hello",
				Permission:    &qqbotsdk.Permission{Type: qqbotsdk.PermissionTypeEveryone},
				UnsupportTips: "请升级 QQ 客户端",
			},
		}},
	}})

	t.Log("three messages sent, waiting for clicks (up to five minutes)")
	t.Log("click order: A1, A2, A2 again, A3, B1, B2, C1")

	// Collect until every expected click has arrived and the operator has gone
	// quiet, so a quick run does not sit waiting for the full window.
	expected := map[string]int{"a1": 1, "a2": 2, "a3": 1, "b1": 1}
	deadline := time.After(6 * time.Minute)

	lastCount := 0
	quietSince := time.Now()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

collect:
	for {
		select {
		case <-ticker.C:
			mu.Lock()
			snapshot := append([]string(nil), clicks...)
			mu.Unlock()

			if len(snapshot) != lastCount {
				lastCount = len(snapshot)
				quietSince = time.Now()
				t.Logf("so far %d clicks: %v", len(snapshot), snapshot)
				continue
			}
			if !coversEveryClick(snapshot, expected) || time.Since(quietSince) < 30*time.Second {
				continue
			}
			t.Log("every expected click has arrived and nothing new for 30 seconds")
			break collect
		case <-deadline:
			t.Log("the six minute window passed, stopping collection")
			break collect
		}
	}

	mu.Lock()
	defer mu.Unlock()
	t.Logf("collected %d clicks in order: %v", len(clicks), clicks)

	counts := map[string]int{}
	for _, button := range clicks {
		counts[button]++
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("  %s clicked %d time(s)", name, counts[name])
	}

	if counts["a2"] < 2 {
		t.Log("A2 arrived fewer than twice: either it greyed out after the failed " +
			"answer, or the second click was not attempted")
	} else {
		t.Log("A2 arrived twice, so a failed answer leaves the button clickable")
	}
	if counts["b2"] > 0 {
		t.Log("B2 arrived, so a shared group id did NOT grey out its sibling")
	} else {
		t.Log("B2 never arrived, consistent with the documented group greying")
	}
	if len(clicks) == 0 || (counts["c1"] == 0 && counts["a1"] > 0) {
		t.Log("C1 produced no interaction, consistent with a command button " +
			"inserting text instead of reporting a click")
	}
}

// coversEveryClick reports whether the collected clicks cover every expected
// count, so the collector can stop once the operator is done.
func coversEveryClick(clicks []string, expected map[string]int) bool {
	counts := map[string]int{}
	for _, button := range clicks {
		counts[button]++
	}
	for button, want := range expected {
		if counts[button] < want {
			return false
		}
	}
	return true
}

// callbackButton builds a callback button, optionally in a greying group.
func callbackButton(id, label, data, groupID string) qqbotsdk.Button {
	return qqbotsdk.Button{
		ID: id,
		RenderData: &qqbotsdk.RenderData{
			Label:        label,
			VisitedLabel: label,
			Style:        qqbotsdk.KeyboardStyleBlue,
		},
		Action: &qqbotsdk.Action{
			Type:          qqbotsdk.ActionTypeCallback,
			Data:          data,
			Permission:    &qqbotsdk.Permission{Type: qqbotsdk.PermissionTypeEveryone},
			UnsupportTips: "请升级 QQ 客户端",
		},
		GroupID: groupID,
	}
}
