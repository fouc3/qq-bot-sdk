//go:build production

package qqbotsdk_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionJoinRequestApproval lists the pending join requests and answers
// one of them.
//
// Which answer is used comes from QQBOT_APPROVAL_OP, defaulting to approve
// because that is the path that also puts the applicant back in the group. A
// decline can be tried by setting it to decline.
func TestProductionJoinRequestApproval(t *testing.T) {
	cfg := loadProductionConfig(t)
	groupOpenID := groupTarget(t)
	client := productionClient(cfg)

	op := os.Getenv("QQBOT_APPROVAL_OP")
	if op == "" {
		op = qqbotsdk.JoinApprovalApprove
	}
	if op != qqbotsdk.JoinApprovalApprove && op != qqbotsdk.JoinApprovalDecline {
		t.Fatalf("QQBOT_APPROVAL_OP must be %s or %s, got %q",
			qqbotsdk.JoinApprovalApprove, qqbotsdk.JoinApprovalDecline, op)
	}
	wantMember := os.Getenv("QQBOT_TEST_MEMBER_OPENID")

	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()

	before, err := client.GetGroupInfo(ctx, groupOpenID)
	if err != nil {
		t.Fatalf("GetGroupInfo before: %v", err)
	}
	t.Logf("before: %q has %d member(s)", before.GroupName, before.GroupMemberNum)

	requests, err := client.ListGroupJoinRequests(ctx, groupOpenID, "", 20)
	if err != nil {
		t.Fatalf("ListGroupJoinRequests: %v", err)
	}
	t.Logf("pending requests: %d", len(requests.List))
	for _, request := range requests.List {
		t.Logf("  id=%s member=%s username=%q source=%s invited_by=%q bot=%v risk=%q",
			request.JoinRequestID, request.MemberOpenID, request.Username,
			request.ApplySource, request.InvitedBy, request.Bot, request.RiskTips)
		if request.VerifyInfo != nil {
			t.Logf("    verify method=%q message=%q questions=%d",
				request.VerifyInfo.Method, request.VerifyInfo.VerifyMessage,
				len(request.VerifyInfo.ReviewQAList))
		}
	}
	if len(requests.List) == 0 {
		t.Skip("there is no pending join request to answer")
	}

	// Prefer the named member, otherwise answer the only request.
	target := requests.List[0]
	if wantMember != "" {
		found := false
		for _, request := range requests.List {
			if request.MemberOpenID == wantMember {
				target = request
				found = true
			}
		}
		if !found {
			t.Skipf("no pending request from %s among %d", wantMember, len(requests.List))
		}
	}
	t.Logf("answering the request from %q (%s) with %s",
		target.Username, target.MemberOpenID, op)

	err = client.ApproveGroupJoinRequest(ctx, groupOpenID, target.MemberOpenID,
		&qqbotsdk.JoinRequestApproval{
			Op:            op,
			JoinRequestID: target.JoinRequestID,
		})
	if err != nil {
		t.Fatalf("ApproveGroupJoinRequest(%s): %v", op, err)
	}
	t.Logf("the request was answered with %s", op)

	// The answered request must leave the queue.
	after, err := client.ListGroupJoinRequests(ctx, groupOpenID, "", 20)
	if err != nil {
		t.Fatalf("ListGroupJoinRequests after: %v", err)
	}
	for _, request := range after.List {
		if request.MemberOpenID == target.MemberOpenID {
			t.Errorf("the answered request from %s is still pending", request.MemberOpenID)
		}
	}
	t.Logf("pending requests now: %d", len(after.List))

	if op == qqbotsdk.JoinApprovalApprove {
		// An approval is only real if the member count goes up.
		var info *qqbotsdk.GroupInfo
		for attempt := 1; attempt <= 6; attempt++ {
			info, err = client.GetGroupInfo(ctx, groupOpenID)
			if err != nil {
				t.Fatalf("GetGroupInfo after: %v", err)
			}
			if info.GroupMemberNum > before.GroupMemberNum {
				break
			}
			t.Logf("the count is still %d, waiting for it to settle (attempt %d)",
				info.GroupMemberNum, attempt)
			time.Sleep(5 * time.Second)
		}
		t.Logf("after: %d member(s)", info.GroupMemberNum)
		if info.GroupMemberNum <= before.GroupMemberNum {
			t.Errorf("the member count did not rise: %d before, %d after",
				before.GroupMemberNum, info.GroupMemberNum)
		} else {
			t.Logf("the member count rose by %d, so %q rejoined",
				info.GroupMemberNum-before.GroupMemberNum, target.Username)
		}
		return
	}

	t.Logf("declined %q%s", target.Username,
		strings.TrimSpace(" with no rejection reason sent"))
}
