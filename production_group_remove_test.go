//go:build production

package qqbotsdk_test

import (
	"context"
	"os"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionBatchRemoveMember removes one member from the group and checks
// that the member count fell.
//
// This call cannot be undone: the bot has no way to add a member back, so the
// account leaves the group for good and a person has to re-invite it. It
// therefore runs only when QQBOT_CONFIRM_REMOVE says so.
//
// The blacklist flag is deliberately left off. Blacklisting is refused by an
// application permission here (40012010), and since lifting a blacklist is the
// same refused endpoint, a member that did get blacklisted could not be
// released again by this bot.
func TestProductionBatchRemoveMember(t *testing.T) {
	if os.Getenv("QQBOT_CONFIRM_REMOVE") == "" {
		t.Skip("set QQBOT_CONFIRM_REMOVE=1 to remove a member; the bot cannot add it back")
	}

	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	memberOpenID := os.Getenv("QQBOT_TEST_MEMBER_OPENID")
	if memberOpenID == "" {
		t.Skip("set QQBOT_TEST_MEMBER_OPENID to the member to remove")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()

	before, err := client.GetGroupInfo(ctx, groupOpenID)
	if err != nil {
		t.Fatalf("GetGroupInfo before: %v", err)
	}
	t.Logf("before: %q has %d member(s)", before.GroupName, before.GroupMemberNum)

	result, err := client.BatchRemoveGroupMembers(ctx, groupOpenID, &qqbotsdk.BatchRemoveMembersRequest{
		MemberOpenIDs:        []string{memberOpenID},
		AddToMemberBlacklist: false,
	})
	if err != nil {
		// The removal endpoint sits behind the same application permission as
		// the member list, so a refusal is the platform saying no rather than a
		// defect, and the account simply stays in the group.
		if qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrGroupNoAPIPermission) {
			t.Skipf("the platform does not let this bot remove members: %v", err)
		}
		t.Fatalf("BatchRemoveGroupMembers: %v", err)
	}
	t.Logf("removal answered: result=%q blacklist_failures=%v",
		result.Result, result.BlacklistFailedOpenIDs)

	// The count is the only confirmation available: the member list is behind a
	// permission this bot lacks, so a removal cannot be read back member by
	// member.
	var after *qqbotsdk.GroupInfo
	for attempt := 1; attempt <= 6; attempt++ {
		after, err = client.GetGroupInfo(ctx, groupOpenID)
		if err != nil {
			t.Fatalf("GetGroupInfo after: %v", err)
		}
		if after.GroupMemberNum < before.GroupMemberNum {
			break
		}
		t.Logf("the count is still %d, waiting for it to settle (attempt %d)",
			after.GroupMemberNum, attempt)
		time.Sleep(5 * time.Second)
	}
	t.Logf("after: %d member(s)", after.GroupMemberNum)

	if after.GroupMemberNum >= before.GroupMemberNum {
		t.Errorf("the member count did not fall: %d before, %d after",
			before.GroupMemberNum, after.GroupMemberNum)
	} else {
		t.Logf("the member count fell by %d, so the removal took effect",
			before.GroupMemberNum-after.GroupMemberNum)
	}

	// Removing the same member again is expected to be refused or to report
	// nothing to do; either way the account must not come back.
	if _, err := client.BatchRemoveGroupMembers(ctx, groupOpenID, &qqbotsdk.BatchRemoveMembersRequest{
		MemberOpenIDs: []string{memberOpenID},
	}); err != nil {
		t.Logf("removing an absent member answered %v", err)
	} else {
		t.Log("removing an absent member was accepted silently")
	}
}
