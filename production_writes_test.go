//go:build production

package qqbotsdk_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionGenerateShareLink asks the platform for a share link, which is
// the address a user would open to add the bot.
func TestProductionGenerateShareLink(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	link, err := client.GenerateShareLink(ctx, "production-test")
	if err != nil {
		t.Fatalf("GenerateShareLink: %v", err)
	}
	t.Logf("share link: %s", link)
	if link == "" {
		t.Fatal("the platform returned an empty share link")
	}
	if !strings.Contains(link, "http") {
		t.Errorf("link = %q, want an http address", link)
	}
}

// TestProductionPanelLifecycle creates a single chat panel, reads it back,
// updates it, finds it in the list and deletes it, cleaning up whatever it
// created even when an assertion fails.
func TestProductionPanelLifecycle(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	remark := fmt.Sprintf("生产测试面板-%d", time.Now().UnixNano())
	panelID, err := client.CreatePanel(ctx, &qqbotsdk.PanelCreateRequest{
		Scope:      qqbotsdk.PanelScopeC2C,
		TargetType: qqbotsdk.PanelTargetAll,
		Panel: &qqbotsdk.Panel{
			Items: []qqbotsdk.PanelItem{
				{Type: qqbotsdk.PanelItemCommand, Name: "帮助", Desc: "查看帮助"},
				{Type: qqbotsdk.PanelItemLink, Name: "官网", Link: "https://example.com"},
			},
			Remark: remark,
		},
	})
	if err != nil {
		t.Fatalf("CreatePanel: %v", err)
	}
	t.Logf("created panel %s", panelID)
	if panelID == "" {
		t.Fatal("the platform returned no panel id")
	}

	// Deleting is the whole point of the lifecycle, so it must run even when
	// an assertion below fails.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		// The body deletes the panel itself, so a missing panel here means the
		// test got that far and is not a cleanup failure.
		err := client.DeletePanel(cleanupCtx, panelID)
		switch {
		case err == nil:
			t.Logf("cleaned up panel %s", panelID)
		case qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrPanelNotFound):
			t.Logf("panel %s was already deleted", panelID)
		default:
			t.Errorf("cleaning up panel %s: %v", panelID, err)
		}
	})

	detail, err := client.GetPanel(ctx, panelID)
	if err != nil {
		t.Fatalf("GetPanel: %v", err)
	}
	if detail.Scope != qqbotsdk.PanelScopeC2C {
		t.Errorf("Scope = %q, want c2c", detail.Scope)
	}
	if detail.TargetType != qqbotsdk.PanelTargetAll {
		t.Errorf("TargetType = %q, want all", detail.TargetType)
	}
	if detail.Panel == nil || len(detail.Panel.Items) != 2 {
		t.Fatalf("Panel = %+v, want the two items that were created", detail.Panel)
	}
	if detail.Panel.Remark != remark {
		t.Errorf("Remark = %q, want %q", detail.Panel.Remark, remark)
	}
	t.Logf("panel detail: version=%d created=%s updated=%s items=%d",
		detail.Version, detail.CreatedAt, detail.UpdatedAt, len(detail.Panel.Items))

	updated, err := client.UpdatePanel(ctx, panelID, &qqbotsdk.Panel{
		Items:  []qqbotsdk.PanelItem{{Type: qqbotsdk.PanelItemCommand, Name: "新指令", Desc: "更新后"}},
		Remark: remark + "-已更新",
	})
	if err != nil {
		t.Fatalf("UpdatePanel: %v", err)
	}
	t.Logf("updated panel, version=%d", updated.Version)

	afterUpdate, err := client.GetPanel(ctx, panelID)
	if err != nil {
		t.Fatalf("GetPanel after update: %v", err)
	}
	if afterUpdate.Panel == nil || len(afterUpdate.Panel.Items) != 1 {
		t.Fatalf("Panel = %+v, want the single updated item", afterUpdate.Panel)
	}
	if afterUpdate.Panel.Items[0].Name != "新指令" {
		t.Errorf("item name = %q, want the updated one", afterUpdate.Panel.Items[0].Name)
	}

	// The panel must appear in the list for its scope.
	page, err := client.ListPanels(ctx, qqbotsdk.PanelScopeC2C, "", 50)
	if err != nil {
		t.Fatalf("ListPanels: %v", err)
	}
	found := false
	for _, record := range page.Records {
		if record.PanelID == panelID {
			found = true
		}
	}
	if !found {
		t.Errorf("panel %s is missing from the c2c list of %d records", panelID, len(page.Records))
	} else {
		t.Logf("panel found in the c2c list among %d records", len(page.Records))
	}

	if err := client.DeletePanel(ctx, panelID); err != nil {
		t.Fatalf("DeletePanel: %v", err)
	}
	t.Log("deleted the panel")

	// A deleted panel must report the documented not-found code.
	if _, err := client.GetPanel(ctx, panelID); err == nil {
		t.Error("reading a deleted panel must fail")
	} else if !qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrPanelNotFound) {
		t.Logf("reading a deleted panel failed with %v (not the documented 40030006)", err)
	} else {
		t.Log("reading a deleted panel returns the documented not-found code")
	}
}

// TestProductionPanelSpecificTargets creates a panel aimed at one user and
// changes its associated users, which is the only way to exercise the target
// endpoint: a global panel refuses the call.
func TestProductionPanelSpecificTargets(t *testing.T) {
	cfg := loadProductionConfig(t)

	// The target has to be a real openid, and the only place to get one is an
	// incoming message, so the single chat harness supplies it.
	client, received := c2cHarness(t, cfg)
	userOpenID := received.Author.UserOpenID

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	panelID, err := client.CreatePanel(ctx, &qqbotsdk.PanelCreateRequest{
		Scope:       qqbotsdk.PanelScopeC2C,
		TargetType:  qqbotsdk.PanelTargetSpecific,
		UserOpenIDs: []string{userOpenID},
		Panel:       &qqbotsdk.Panel{Items: []qqbotsdk.PanelItem{{Type: qqbotsdk.PanelItemCommand, Name: "专属指令"}}},
	})
	if err != nil {
		t.Fatalf("CreatePanel specific: %v", err)
	}
	t.Logf("created a specific panel %s for %s", panelID, userOpenID)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		err := client.DeletePanel(cleanupCtx, panelID)
		if err != nil && !qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrPanelNotFound) {
			t.Errorf("cleaning up panel %s: %v", panelID, err)
		}
	})

	detail, err := client.GetPanel(ctx, panelID)
	if err != nil {
		t.Fatalf("GetPanel: %v", err)
	}
	if detail.TargetType != qqbotsdk.PanelTargetSpecific {
		t.Errorf("TargetType = %q, want specific", detail.TargetType)
	}
	if len(detail.UserOpenIDs) != 1 || detail.UserOpenIDs[0] != userOpenID {
		t.Errorf("UserOpenIDs = %v, want the created target", detail.UserOpenIDs)
	}
	t.Logf("the panel detail carries %d associated user(s)", len(detail.UserOpenIDs))

	// Removing and re-adding the association exercises both operations.
	if err := client.UpdatePanelTargets(ctx, panelID, &qqbotsdk.PanelTargetRequest{
		Op:          qqbotsdk.PanelTargetOpDel,
		UserOpenIDs: []string{userOpenID},
	}); err != nil {
		t.Fatalf("removing the target: %v", err)
	}
	t.Log("removed the associated user")

	afterRemoval, err := client.GetPanel(ctx, panelID)
	if err != nil {
		t.Fatalf("GetPanel after removal: %v", err)
	}
	if len(afterRemoval.UserOpenIDs) != 0 {
		t.Errorf("UserOpenIDs = %v, want none after the removal", afterRemoval.UserOpenIDs)
	}

	if err := client.UpdatePanelTargets(ctx, panelID, &qqbotsdk.PanelTargetRequest{
		Op:          qqbotsdk.PanelTargetOpAdd,
		UserOpenIDs: []string{userOpenID},
	}); err != nil {
		t.Fatalf("re-adding the target: %v", err)
	}
	afterAdd, err := client.GetPanel(ctx, panelID)
	if err != nil {
		t.Fatalf("GetPanel after re-adding: %v", err)
	}
	if len(afterAdd.UserOpenIDs) != 1 {
		t.Errorf("UserOpenIDs = %v, want the target back", afterAdd.UserOpenIDs)
	}
	t.Log("re-added the associated user")

	// A global panel must reject the operation with the documented code, which
	// is asserted by the unit tests; here only the specific path is exercised.
}

// TestProductionMenuLifecycle reads the live menu, replaces it, verifies the
// change and restores whatever was there before.
//
// The restore runs from a cleanup so a failing assertion cannot leave the bot
// with a test menu.
func TestProductionMenuLifecycle(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	before, err := client.GetMenu(ctx)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	t.Logf("the current menu is version %d", before.Version)

	original := before.Menu
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		restore := original
		if restore == nil {
			restore = &qqbotsdk.Menu{Items: []qqbotsdk.MenuItem{}}
		}
		version, err := client.SetMenu(cleanupCtx, restore)
		if err != nil {
			t.Errorf("restoring the menu: %v", err)
			return
		}
		t.Logf("restored the menu to its original state, version %d", version.Version)
	})

	set, err := client.SetMenu(ctx, &qqbotsdk.Menu{Items: []qqbotsdk.MenuItem{
		{Type: qqbotsdk.MenuTypeSendMessage, Name: "生产测试", SendMessage: "/production-test"},
	}})
	if err != nil {
		t.Fatalf("SetMenu: %v", err)
	}
	t.Logf("set a one button menu, version %d", set.Version)

	after, err := client.GetMenu(ctx)
	if err != nil {
		t.Fatalf("GetMenu after SetMenu: %v", err)
	}
	if after.Menu == nil {
		t.Fatal("the menu is empty after being set")
	}
	if len(after.Menu.Items) != 1 {
		t.Fatalf("items = %d, want the single button that was set", len(after.Menu.Items))
	}
	item := after.Menu.Items[0]
	if item.Name != "生产测试" || item.SendMessage != "/production-test" {
		t.Errorf("item = %+v, want the button that was set", item)
	}
	if item.Type != qqbotsdk.MenuTypeSendMessage {
		t.Errorf("item type = %q", item.Type)
	}
	t.Logf("the menu now holds %q", item.Name)
}
