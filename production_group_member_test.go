//go:build production

package qqbotsdk_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionMuteCapturedMember captures a member openid from a live group
// message and then changes that member's mute.
//
// A member openid cannot be discovered through the API here: the member list
// and the single member call are gated behind a permission this bot lacks
// (40012010). The only way to learn one is a message event, so the test waits
// for the named account to speak in the group and reads it from there.
//
// The mute is always lifted again, from a cleanup, so a failure part way
// through does not leave the account muted.
func TestProductionMuteCapturedMember(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	wantName := os.Getenv("QQBOT_TEST_MEMBER_NAME")
	if wantName == "" {
		wantName = "btrfs"
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	gatewayURL, err := cachedGateway(ctx, client)
	if err != nil {
		t.Fatalf("GetGateway: %v", err)
	}

	type speaker struct {
		openID   string
		username string
		role     string
		bot      bool
	}
	var (
		mu   sync.Mutex
		seen []speaker
	)
	ready := make(chan struct{}, 1)

	client.RegisterFunc(qqbotsdk.EventReady, func(_ context.Context, _ *qqbotsdk.Event) error {
		select {
		case ready <- struct{}{}:
		default:
		}
		return nil
	})
	client.RegisterFunc(qqbotsdk.EventGroupMessageCreate, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			t.Errorf("decoding GROUP_MESSAGE_CREATE: %v", err)
			return err
		}
		data := value.(*qqbotsdk.GroupMessageCreateData)
		if data.GroupOpenID != groupOpenID || data.Author == nil {
			return nil
		}
		entry := speaker{
			openID:   data.Author.MemberOpenID,
			username: data.Author.Username,
			role:     data.Author.MemberRole,
			bot:      data.Author.Bot,
		}
		mu.Lock()
		seen = append(seen, entry)
		mu.Unlock()
		t.Logf("group message from openid=%s username=%q role=%q bot=%v content=%q",
			entry.openID, entry.username, entry.role, entry.bot, data.Content)
		return nil
	})

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
	t.Logf("listening for a message from %q in group %s — send one now", wantName, groupOpenID)

	// Wait for the named account to speak, matching the nickname loosely since
	// a group event does not always carry one.
	var target speaker
	waitUntil := time.Now().Add(8 * time.Minute)
	lastReport := time.Now()
	for target.openID == "" && time.Now().Before(waitUntil) {
		mu.Lock()
		for _, entry := range seen {
			if strings.EqualFold(entry.username, wantName) {
				target = entry
			}
		}
		count := len(seen)
		mu.Unlock()

		if target.openID != "" {
			break
		}
		if time.Since(lastReport) >= 30*time.Second {
			lastReport = time.Now()
			t.Logf("still waiting for %q; %d message(s) seen so far", wantName, count)
		}
		time.Sleep(2 * time.Second)
	}

	// Falling back keeps the test useful when the event carries no nickname.
	if target.openID == "" {
		mu.Lock()
		for _, entry := range seen {
			if !entry.bot {
				target = entry
				break
			}
		}
		mu.Unlock()
		if target.openID == "" {
			t.Fatal("no usable group message arrived within eight minutes")
		}
		t.Logf("%q never spoke, falling back to %q", wantName, target.username)
	}
	t.Logf("target member: openid=%s username=%q role=%q bot=%v",
		target.openID, target.username, target.role, target.bot)

	if target.bot {
		t.Skipf("%q is a bot, which the documentation excludes from muting", target.username)
	}
	if target.role != qqbotsdk.GroupRoleMember && target.role != "" {
		t.Skipf("%q is a %q, and only an ordinary member can be muted", target.username, target.role)
	}

	ctx2, cancel2 := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel2()

	// Lifting the mute runs first, so a failure in the assertions below still
	// leaves the account able to speak.
	unmuted := false
	t.Cleanup(func() {
		if unmuted {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		if err := client.SetGroupMemberMute(cleanupCtx, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
			Members: []qqbotsdk.SetMemberMuteState{{
				Op:           qqbotsdk.MemberMuteDelete,
				MemberOpenID: target.openID,
			}},
		}); err != nil {
			t.Errorf("lifting the mute from the cleanup: %v", err)
		} else {
			t.Log("the cleanup lifted the mute")
		}
	})

	expiry := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	if err := client.SetGroupMemberMute(ctx2, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteAdd,
			MemberOpenID: target.openID,
			MuteExpireAt: expiry,
		}},
	}); err != nil {
		t.Fatalf("muting %q: %v", target.username, err)
	}
	t.Logf("MUTED %q until %s", target.username, expiry)

	setting, err := client.GetGroupRestrictChatSetting(ctx2, groupOpenID)
	if err != nil {
		t.Fatalf("GetGroupRestrictChatSetting: %v", err)
	}
	if !muteListed(setting, target.openID) {
		t.Errorf("the muted member is missing from a mute query listing %d member(s)",
			len(setting.Members))
	} else {
		t.Log("the mute query reports the muted member")
	}

	if err := client.SetGroupMemberMute(ctx2, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteDelete,
			MemberOpenID: target.openID,
		}},
	}); err != nil {
		t.Fatalf("unmuting %q: %v", target.username, err)
	}
	unmuted = true
	t.Logf("UNMUTED %q", target.username)

	// An update of an existing mute is the third documented operation.
	if err := client.SetGroupMemberMute(ctx2, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteAdd,
			MemberOpenID: target.openID,
			MuteExpireAt: expiry,
		}},
	}); err != nil {
		t.Fatalf("re-muting: %v", err)
	}
	later := time.Now().Add(20 * time.Minute).Format(time.RFC3339)
	if err := client.SetGroupMemberMute(ctx2, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteUpdate,
			MemberOpenID: target.openID,
			MuteExpireAt: later,
		}},
	}); err != nil {
		t.Fatalf("updating the mute: %v", err)
	}
	t.Logf("the mute was updated to expire at %s", later)

	// Leave the account unmuted whatever happens after this.
	if err := client.SetGroupMemberMute(ctx2, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteDelete,
			MemberOpenID: target.openID,
		}},
	}); err != nil {
		t.Fatalf("the final unmute: %v", err)
	}
	t.Logf("final state: %q is not muted", target.username)

	// Blacklisting a member who is still in the group is documented to fail.
	_, err = client.UpdateGroupBlacklist(ctx2, groupOpenID, &qqbotsdk.UpdateBlacklistRequest{
		Op:            qqbotsdk.ListOpAdd,
		MemberOpenIDs: []string{target.openID},
	})
	if err == nil {
		_, undoErr := client.UpdateGroupBlacklist(ctx2, groupOpenID, &qqbotsdk.UpdateBlacklistRequest{
			Op:            qqbotsdk.ListOpDelete,
			MemberOpenIDs: []string{target.openID},
		})
		t.Logf("the platform ACCEPTED blacklisting a member who is in the group; undo answered %v", undoErr)
	} else {
		t.Logf("blacklisting a member who is in the group was refused, as documented: %v", err)
	}
}

// muteListed reports whether the mute query includes the member.
func muteListed(setting *qqbotsdk.GroupRestrictChatSetting, memberOpenID string) bool {
	if setting == nil {
		return false
	}
	for _, entry := range setting.Members {
		if entry.MemberOpenID == memberOpenID {
			return true
		}
	}
	return false
}
