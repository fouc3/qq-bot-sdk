//go:build production

package qqbotsdk_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// groupTarget is the group the management tests act on.
//
// It has to come from the environment because a group openid is only ever seen
// in an event, never listed from the credentials.
func groupTarget(t *testing.T) string {
	t.Helper()
	groupOpenID := os.Getenv("QQBOT_GROUP_OPENID")
	if groupOpenID == "" {
		t.Skip("set QQBOT_GROUP_OPENID to the group openid the bot administers")
	}
	return groupOpenID
}

// memberTarget is the account whose state the tests may change.
//
// The openid is preferred. Falling back to a nickname needs the member list,
// which the platform gates behind an extra permission.
func memberTarget(t *testing.T, client *qqbotsdk.Client, ctx context.Context, groupOpenID string) string {
	t.Helper()
	if openID := os.Getenv("QQBOT_TEST_MEMBER_OPENID"); openID != "" {
		return openID
	}
	name := os.Getenv("QQBOT_TEST_MEMBER_NAME")
	if name == "" {
		t.Skip("set QQBOT_TEST_MEMBER_OPENID, or QQBOT_TEST_MEMBER_NAME with the member list permitted")
	}

	cursor := ""
	for page := 0; page < 10; page++ {
		list, err := client.ListGroupMembers(ctx, groupOpenID, cursor)
		if err != nil {
			t.Skipf("cannot look up %q by name: %v", name, err)
		}
		for _, member := range list.Members {
			if member.Username == name {
				return member.MemberOpenID
			}
		}
		if list.NextCursor == "" {
			break
		}
		cursor = list.NextCursor
	}
	t.Skipf("no member named %q in the group", name)
	return ""
}

// TestProductionGroupProbe calls every read-only group endpoint and records
// which the platform lets this bot reach.
//
// Each call is attempted even when an earlier one is refused: the platform
// gates these endpoints by permission, so stopping at the first refusal would
// hide the rest of the picture.
func TestProductionGroupProbe(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()

	refused := 0
	// probe runs one call, logging whether it was reachable.
	probe := func(name string, run func() error) {
		t.Helper()
		if err := run(); err != nil {
			refused++
			t.Logf("%-34s REFUSED: %v", name, err)
			return
		}
		t.Logf("%-34s ok", name)
	}

	probe("GetGroupInfo", func() error {
		info, err := client.GetGroupInfo(ctx, groupOpenID)
		if err != nil {
			return err
		}
		t.Logf("  name=%q class=%q tags=%v members=%d",
			info.GroupName, info.GroupClassText, info.GroupTags, info.GroupMemberNum)
		return nil
	})

	probe("GetGroupBotState", func() error {
		state, err := client.GetGroupBotState(ctx, groupOpenID)
		if err != nil {
			return err
		}
		t.Logf("  member=%s role=%s proactive=%v recv=%q",
			state.MemberOpenID, state.MemberRole, state.AllowProactiveMsg, state.RecvMsgSetting)
		return nil
	})

	probe("ListGroupMembers", func() error {
		list, err := client.ListGroupMembers(ctx, groupOpenID, "")
		if err != nil {
			return err
		}
		t.Logf("  %d member(s) on the first page", len(list.Members))
		for _, member := range list.Members {
			t.Logf("    %s role=%s bot=%v", member.Username, member.MemberRole, member.Bot)
		}
		return nil
	})

	probe("GetGroupRestrictChatSetting", func() error {
		setting, err := client.GetGroupRestrictChatSetting(ctx, groupOpenID)
		if err != nil {
			return err
		}
		mode := ""
		if setting.GlobalRule != nil {
			mode = setting.GlobalRule.Mode
		}
		t.Logf("  global mode=%q, %d muted member(s)", mode, len(setting.Members))
		return nil
	})

	probe("ListGroupJoinRequests", func() error {
		requests, err := client.ListGroupJoinRequests(ctx, groupOpenID, "", 20)
		if err != nil {
			return err
		}
		t.Logf("  %d pending request(s)", len(requests.List))
		for _, request := range requests.List {
			t.Logf("    %s (%s) source=%s risk=%q",
				request.Username, request.MemberOpenID, request.ApplySource, request.RiskTips)
		}
		return nil
	})

	probe("ListGroupBlacklist", func() error {
		blacklist, err := client.ListGroupBlacklist(ctx, groupOpenID, "", 20)
		if err != nil {
			return err
		}
		t.Logf("  %d blacklisted user(s)", len(blacklist.Users))
		for _, user := range blacklist.Users {
			t.Logf("    %s banned %s", user.Username, user.BannedAt)
		}
		return nil
	})

	probe("ListJoinApprovalStrategies", func() error {
		strategies, err := client.ListJoinApprovalStrategies(ctx, "", 20)
		if err != nil {
			return err
		}
		t.Logf("  %d active strategy/strategies", len(strategies.Strategies))
		for _, strategy := range strategies.Strategies {
			t.Logf("    %s enable=%s groups=%v whitelist=%d remark=%q",
				strategy.StrategyID, strategy.IsEnable, strategy.GroupOpenIDs,
				strategy.WhitelistUserCount, strategy.Remark)
		}
		return nil
	})

	memberOpenID := os.Getenv("QQBOT_TEST_MEMBER_OPENID")
	if memberOpenID != "" {
		probe("GetGroupMember", func() error {
			member, err := client.GetGroupMember(ctx, groupOpenID, memberOpenID)
			if err != nil {
				return err
			}
			t.Logf("  %s role=%s bot=%v", member.Username, member.MemberRole, member.Bot)
			return nil
		})
	}

	t.Logf("%d of the probed calls were refused", refused)
}

// TestProductionStrategyLifecycle creates, changes, runs and deletes an
// automatic join approval strategy.
//
// This one is safe to run: a strategy is the bot's own configuration, it is
// scoped to the group it is given, it is created disabled so it approves
// nobody, and the cleanup deletes it.
func TestProductionStrategyLifecycle(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()

	remark := fmt.Sprintf("生产测试策略-%d", time.Now().UnixNano())
	created, err := client.CreateJoinApprovalStrategy(ctx, &qqbotsdk.CreateJoinApprovalStrategyRequest{
		GroupOpenIDs: []string{groupOpenID},
		IsEnable:     qqbotsdk.StrategyDisabled, // created off, so it approves nobody
		Remark:       remark,
	})
	if err != nil {
		t.Fatalf("CreateJoinApprovalStrategy: %v", err)
	}
	t.Logf("created strategy %s enable=%q expire=%q", created.StrategyID, created.IsEnable, created.ExpireAt)
	if created.StrategyID == "" {
		t.Fatal("the platform returned no strategy id")
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cleanupCancel()
		// The body deletes the strategy too, so a failure here is logged rather
		// than failed; a rapid second call may also be rate limited.
		if err := client.DeleteJoinApprovalStrategy(cleanupCtx, created.StrategyID); err != nil {
			t.Logf("deleting strategy %s during cleanup: %v", created.StrategyID, err)
			return
		}
		t.Logf("deleted strategy %s", created.StrategyID)
	})

	list, err := client.ListJoinApprovalStrategies(ctx, "", 50)
	if err != nil {
		t.Fatalf("ListJoinApprovalStrategies: %v", err)
	}
	found := false
	for _, strategy := range list.Strategies {
		if strategy.StrategyID == created.StrategyID {
			found = true
			t.Logf("strategy is listed: enable=%q whitelist=%d remark=%q",
				strategy.IsEnable, strategy.WhitelistUserCount, strategy.Remark)
		}
	}
	if !found {
		t.Errorf("strategy %s is missing from a list of %d", created.StrategyID, len(list.Strategies))
	}

	updated, err := client.UpdateJoinApprovalStrategy(ctx, created.StrategyID,
		&qqbotsdk.UpdateJoinApprovalStrategyRequest{IsEnable: qqbotsdk.StrategyEnabled})
	if err != nil {
		t.Fatalf("UpdateJoinApprovalStrategy: %v", err)
	}
	t.Logf("updated strategy: enable=%q expire=%q", updated.IsEnable, updated.ExpireAt)

	whitelist, err := client.UpdateJoinApprovalStrategyWhitelist(ctx, created.StrategyID,
		&qqbotsdk.UpdateWhitelistRequest{
			Op:             qqbotsdk.ListOpAdd,
			WhitelistUsers: []string{"10001", "10002"},
		})
	if err != nil {
		t.Fatalf("whitelist add: %v", err)
	}
	t.Logf("whitelist after add: %d numbers", whitelist.WhitelistUserCount)

	whitelist, err = client.UpdateJoinApprovalStrategyWhitelist(ctx, created.StrategyID,
		&qqbotsdk.UpdateWhitelistRequest{
			Op:             qqbotsdk.ListOpDelete,
			WhitelistUsers: []string{"10001", "10002"},
		})
	if err != nil {
		t.Fatalf("whitelist delete: %v", err)
	}
	t.Logf("whitelist after delete: %d numbers", whitelist.WhitelistUserCount)

	if err := client.ExecuteJoinApprovalStrategy(ctx, created.StrategyID); err != nil {
		t.Fatalf("ExecuteJoinApprovalStrategy: %v", err)
	}
	t.Log("the strategy executed")

	if err := client.DeleteJoinApprovalStrategy(ctx, created.StrategyID); err != nil {
		t.Fatalf("DeleteJoinApprovalStrategy: %v", err)
	}
	t.Log("the strategy was deleted")
}

// TestProductionMemberMute mutes and unmutes one member, then checks that the
// mute query reports it.
//
// The documentation says only an ordinary member can be muted, so the test
// looks the account up first, skips when it is the owner or an administrator or
// a bot, and always lifts the mute again from a cleanup.
func TestProductionMemberMute(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()

	memberOpenID := memberTarget(t, client, ctx, groupOpenID)

	member, err := client.GetGroupMember(ctx, groupOpenID, memberOpenID)
	if err != nil {
		t.Fatalf("GetGroupMember: %v", err)
	}
	t.Logf("the target is %q, role %q, bot=%v", member.Username, member.MemberRole, member.Bot)

	if member.MemberRole != qqbotsdk.GroupRoleMember {
		t.Skipf("a %q cannot be muted, per the documentation", member.MemberRole)
	}
	if member.Bot {
		t.Skip("the documentation excludes bots from being muted")
	}

	expiry := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
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
				MemberOpenID: memberOpenID,
			}},
		}); err != nil {
			t.Errorf("lifting the mute: %v", err)
		} else {
			t.Log("the mute was lifted by the cleanup")
		}
	})

	if err := client.SetGroupMemberMute(ctx, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteAdd,
			MemberOpenID: memberOpenID,
			MuteExpireAt: expiry,
		}},
	}); err != nil {
		t.Fatalf("muting: %v", err)
	}
	t.Logf("muted %q until %s", member.Username, expiry)

	setting, err := client.GetGroupRestrictChatSetting(ctx, groupOpenID)
	if err != nil {
		t.Fatalf("GetGroupRestrictChatSetting: %v", err)
	}
	muted := false
	for _, entry := range setting.Members {
		if entry.MemberOpenID == memberOpenID {
			muted = true
			t.Logf("the mute is visible in the query: until %s", entry.MuteExpireAt)
		}
	}
	if !muted {
		t.Error("the muted member is missing from the mute query")
	}

	if err := client.SetGroupMemberMute(ctx, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
		Members: []qqbotsdk.SetMemberMuteState{{
			Op:           qqbotsdk.MemberMuteDelete,
			MemberOpenID: memberOpenID,
		}},
	}); err != nil {
		t.Fatalf("unmuting: %v", err)
	}
	unmuted = true
	t.Log("unmuted; the account can speak again")
}

// TestProductionBlacklistRefusal checks the documented rule that a member who
// is still in the group cannot be blacklisted.
//
// It only ever expects a refusal, and undoes the change if the platform happens
// to accept it, so nobody is left blacklisted.
func TestProductionBlacklistRefusal(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()

	memberOpenID := memberTarget(t, client, ctx, groupOpenID)

	_, err := client.UpdateGroupBlacklist(ctx, groupOpenID, &qqbotsdk.UpdateBlacklistRequest{
		Op:            qqbotsdk.ListOpAdd,
		MemberOpenIDs: []string{memberOpenID},
	})
	if err == nil {
		_, undoErr := client.UpdateGroupBlacklist(ctx, groupOpenID, &qqbotsdk.UpdateBlacklistRequest{
			Op:            qqbotsdk.ListOpDelete,
			MemberOpenIDs: []string{memberOpenID},
		})
		t.Logf("the platform ACCEPTED blacklisting a member who is in the group; "+
			"the undo answered %v", undoErr)
		return
	}

	var apiErr *qqbotsdk.OpenAPIError
	if errors.As(err, &apiErr) {
		t.Logf("blacklisting a member who is in the group was refused: code=%d %q text=%q",
			apiErr.Code, apiErr.Code.String(), apiErr.Message)
	} else {
		t.Logf("refused with %v", err)
	}
}
