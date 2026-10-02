//go:build production

package qqbotsdk_test

import (
	"context"
	"os"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// TestProductionJoinRequestApproval lists the pending join requests and answers
// one of them.
//
// Which answer is used comes from QQBOT_APPROVAL_OP, defaulting to approve
// because that is the path that also puts the applicant back in the group.
// Setting it to decline exercises the rejection path, with a rejection reason
// taken from QQBOT_REJECT_REASON.
//
// The request is waited for rather than assumed, so an operator can make the
// account leave and apply again at their own pace after the test starts.
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
	rejectReason := os.Getenv("QQBOT_REJECT_REASON")
	if rejectReason == "" {
		rejectReason = "生产测试：拒绝理由能否送达"
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	before, err := client.GetGroupInfo(ctx, groupOpenID)
	if err != nil {
		t.Fatalf("GetGroupInfo before: %v", err)
	}
	t.Logf("before: %q has %d member(s)", before.GroupName, before.GroupMemberNum)

	// Wait for the request, so the operator does not have to time their action
	// against the test starting.
	var requests *qqbotsdk.JoinRequestList
	waitUntil := time.Now().Add(8 * time.Minute)
	lastReport := time.Now()
	for {
		requests, err = client.ListGroupJoinRequests(ctx, groupOpenID, "", 20)
		if err != nil {
			t.Fatalf("ListGroupJoinRequests: %v", err)
		}
		if len(requests.List) > 0 || time.Now().After(waitUntil) {
			break
		}
		if time.Since(lastReport) >= 30*time.Second {
			lastReport = time.Now()
			t.Log("no pending request yet; make the account leave and apply again")
		}
		time.Sleep(10 * time.Second)
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
		t.Skip("no join request appeared")
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

	approval := &qqbotsdk.JoinRequestApproval{
		Op:            op,
		JoinRequestID: target.JoinRequestID,
	}
	if op == qqbotsdk.JoinApprovalDecline {
		// Blacklisting is left off: lifting one is refused by an application
		// permission here, so a blacklisted account could not be released again.
		approval.RejectReason = rejectReason
		t.Logf("sending the rejection reason %q with the blacklist flag off", rejectReason)
	}
	if err := client.ApproveGroupJoinRequest(ctx, groupOpenID, target.MemberOpenID, approval); err != nil {
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

	// The member count settles the question: approving admits the account and
	// declining keeps it out. It is the only readback available, because the
	// member list is behind a permission this bot lacks.
	var info *qqbotsdk.GroupInfo
	for attempt := 1; attempt <= 6; attempt++ {
		info, err = client.GetGroupInfo(ctx, groupOpenID)
		if err != nil {
			t.Fatalf("GetGroupInfo after: %v", err)
		}
		wantHigher := op == qqbotsdk.JoinApprovalApprove
		if (wantHigher && info.GroupMemberNum > before.GroupMemberNum) ||
			(!wantHigher && info.GroupMemberNum < before.GroupMemberNum) {
			break
		}
		t.Logf("the count is still %d, waiting for it to settle (attempt %d)",
			info.GroupMemberNum, attempt)
		time.Sleep(5 * time.Second)
	}
	t.Logf("after: %d member(s)", info.GroupMemberNum)

	switch {
	case op == qqbotsdk.JoinApprovalApprove && info.GroupMemberNum > before.GroupMemberNum:
		t.Logf("the member count rose by %d, so %q rejoined",
			info.GroupMemberNum-before.GroupMemberNum, target.Username)
	case op == qqbotsdk.JoinApprovalApprove:
		t.Errorf("the member count did not rise: %d before, %d after",
			before.GroupMemberNum, info.GroupMemberNum)
	case info.GroupMemberNum > before.GroupMemberNum:
		// A rejection must never admit anyone, which is the point of checking
		// the count here.
		t.Errorf("the member count rose from %d to %d despite a rejection",
			before.GroupMemberNum, info.GroupMemberNum)
	default:
		t.Logf("the member count held at %d, so the rejection did not admit %q",
			info.GroupMemberNum, target.Username)
	}
}
