package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// groupRequest runs one call against a stub server and returns the captured
// request, so each endpoint test only states what it sends.
func groupRequest(t *testing.T, body string, call func(*Client) error) capturedRequest {
	t.Helper()
	client, captured := newMessageServer(t, http.StatusOK, body)
	if err := call(client); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("sent %d requests, want 1", len(*captured))
	}
	return last(t, captured)
}

// TestGetGroupInfoMatchesDocumentedRequest covers the group information call.
func TestGetGroupInfoMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"group_openid": "G1", "group_name": "读书分享会",
		"group_finger_memo": "每周共读一本好书", "group_class_text": "文化",
		"group_tags": ["阅读", "文学"], "group_member_num": 6
	}`, func(c *Client) error {
		info, err := c.GetGroupInfo(t.Context(), "G1")
		if err != nil {
			return err
		}
		if info.GroupName != "读书分享会" || info.GroupMemberNum != 6 {
			t.Errorf("info = %+v", info)
		}
		if len(info.GroupTags) != 2 {
			t.Errorf("GroupTags = %v", info.GroupTags)
		}
		return nil
	})
	if req.Method != http.MethodGet || req.Path != "/v2/groups/G1/info" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
}

// TestGetGroupBotStateMatchesDocumentedRequest covers the bot standing call.
func TestGetGroupBotStateMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"member_openid": "B1", "joined_at": "2025-06-15T14:30:00+08:00",
		"allow_proactive_msg": false, "recv_msg_setting": "only_mention",
		"member_role": "member"
	}`, func(c *Client) error {
		state, err := c.GetGroupBotState(t.Context(), "G1")
		if err != nil {
			return err
		}
		if state.RecvMsgSetting != GroupRecvMsgOnlyMention {
			t.Errorf("RecvMsgSetting = %q", state.RecvMsgSetting)
		}
		if state.MemberRole != GroupRoleMember || state.AllowProactiveMsg {
			t.Errorf("state = %+v", state)
		}
		return nil
	})
	if req.Method != http.MethodGet || req.Path != "/v2/groups/G1/bot_state" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
}

// TestListGroupJoinRequestsMatchesDocumentedRequest covers the request list,
// including the cursor and the capped page size.
func TestListGroupJoinRequestsMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"list": [{
			"join_request_id": "R1", "risk_tips": "warning_tips",
			"member_openid": "M1", "username": "小明",
			"apply_at": "2026-07-21T10:00:00+08:00", "apply_source": "invited",
			"invited_by": "M9", "bot": false,
			"verify_info": {"method": "admin_review_qa",
				"review_qa_list": [{"question": "学号?", "answer": "12345"}]}
		}],
		"next_cursor": "NEXT"
	}`, func(c *Client) error {
		page, err := c.ListGroupJoinRequests(t.Context(), "G1", "CUR", 999)
		if err != nil {
			return err
		}
		if len(page.List) != 1 || page.NextCursor != "NEXT" {
			t.Fatalf("page = %+v", page)
		}
		request := page.List[0]
		if request.JoinRequestID != "R1" || request.ApplySource != GroupJoinSourceInvited {
			t.Errorf("request = %+v", request)
		}
		if request.VerifyInfo == nil || len(request.VerifyInfo.ReviewQAList) != 1 {
			t.Fatalf("VerifyInfo = %+v", request.VerifyInfo)
		}
		if request.VerifyInfo.ReviewQAList[0].Answer != "12345" {
			t.Errorf("answer = %q", request.VerifyInfo.ReviewQAList[0].Answer)
		}
		return nil
	})
	if req.Path != "/v2/groups/G1/join_request_list" {
		t.Errorf("path = %s", req.Path)
	}
	if !strings.Contains(req.Query, "cursor=CUR") {
		t.Errorf("query = %q, want the cursor", req.Query)
	}
	if !strings.Contains(req.Query, "limit=50") {
		t.Errorf("query = %q, want the limit capped at 50", req.Query)
	}
}

// TestApproveGroupJoinRequestMatchesDocumentedRequest covers the approval call.
func TestApproveGroupJoinRequestMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{}`, func(c *Client) error {
		return c.ApproveGroupJoinRequest(t.Context(), "G1", "M1", &JoinRequestApproval{
			Op:                   JoinApprovalDecline,
			JoinRequestID:        "R1",
			RejectReason:         "满员",
			AddToMemberBlacklist: true,
		})
	})
	if req.Method != http.MethodPost {
		t.Errorf("method = %s", req.Method)
	}
	if req.Path != "/v2/groups/G1/approval_join_request/M1" {
		t.Errorf("path = %s", req.Path)
	}
	body := decodeBody(t, req)
	if body["op"] != JoinApprovalDecline || body["reject_reason"] != "满员" {
		t.Errorf("body = %v", body)
	}
	if body["add_to_member_blacklist"] != true {
		t.Errorf("body = %v, want the blacklist flag", body)
	}
}

// TestGetGroupRestrictChatSettingMatchesDocumentedRequest covers the mute query,
// including the nested schedule and recurring rules.
func TestGetGroupRestrictChatSettingMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"global_rule": {
			"mode": "schedule",
			"schedule_rules": [{"task_id":"T1","start_at":"2026-07-21T10:00:00+08:00",
				"end_at":"2026-07-21T11:00:00+08:00","enabled":true}],
			"recurring_rules": [{"task_id":"T2","weekdays":[1,7],
				"start_time":"23:00","end_time":"06:00","enabled":true}]
		},
		"members": [{"member_openid":"M1","mute_expire_at":"2026-07-22T10:00:00+08:00","username":"小明"}]
	}`, func(c *Client) error {
		setting, err := c.GetGroupRestrictChatSetting(t.Context(), "G1")
		if err != nil {
			return err
		}
		if setting.GlobalRule == nil || setting.GlobalRule.Mode != GroupMuteSchedule {
			t.Fatalf("GlobalRule = %+v", setting.GlobalRule)
		}
		if len(setting.GlobalRule.ScheduleRules) != 1 || len(setting.GlobalRule.RecurringRules) != 1 {
			t.Errorf("rules = %+v", setting.GlobalRule)
		}
		recurring := setting.GlobalRule.RecurringRules[0]
		if len(recurring.Weekdays) != 2 || recurring.StartTime != "23:00" {
			t.Errorf("recurring = %+v", recurring)
		}
		if len(setting.Members) != 1 || setting.Members[0].Username != "小明" {
			t.Errorf("Members = %+v", setting.Members)
		}
		return nil
	})
	if req.Method != http.MethodGet || req.Path != "/v2/groups/G1/restrict_chat_setting" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
}

// TestSetGroupMemberMuteMatchesDocumentedRequest covers the mute call.
func TestSetGroupMemberMuteMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{}`, func(c *Client) error {
		return c.SetGroupMemberMute(t.Context(), "G1", &SetGroupMemberMuteRequest{
			Members: []SetMemberMuteState{
				{Op: MemberMuteAdd, MemberOpenID: "M1", MuteExpireAt: "2026-07-22T10:00:00+08:00"},
				{Op: MemberMuteDelete, MemberOpenID: "M2"},
			},
		})
	})
	if req.Method != http.MethodPost || req.Path != "/v2/groups/G1/restrict_chat_setting" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
	members, ok := decodeBody(t, req)["members"].([]any)
	if !ok || len(members) != 2 {
		t.Fatalf("members = %v", decodeBody(t, req)["members"])
	}
	if members[0].(map[string]any)["op"] != MemberMuteAdd {
		t.Errorf("members = %v", members)
	}
}

// TestListGroupMembersMatchesDocumentedRequest covers the member list.
func TestListGroupMembersMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"members": [{"member_openid":"M1","username":"小明","member_role":"admin",
			"bot":false,"joined_at":"2025-06-15T14:30:00+08:00"}],
		"next_cursor": "NEXT"
	}`, func(c *Client) error {
		page, err := c.ListGroupMembers(t.Context(), "G1", "CUR")
		if err != nil {
			return err
		}
		if len(page.Members) != 1 || page.NextCursor != "NEXT" {
			t.Fatalf("page = %+v", page)
		}
		if page.Members[0].MemberRole != GroupRoleAdmin {
			t.Errorf("member = %+v", page.Members[0])
		}
		return nil
	})
	if req.Path != "/v2/groups/G1/members" || req.Query != "cursor=CUR" {
		t.Errorf("%s?%s", req.Path, req.Query)
	}
}

// TestGetGroupMemberMatchesDocumentedRequest covers the single member call.
func TestGetGroupMemberMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{"member_openid":"M1","username":"小明","member_role":"owner"}`,
		func(c *Client) error {
			member, err := c.GetGroupMember(t.Context(), "G1", "M1")
			if err != nil {
				return err
			}
			if member.MemberRole != GroupRoleOwner {
				t.Errorf("member = %+v", member)
			}
			return nil
		})
	if req.Path != "/v2/groups/G1/members/M1" {
		t.Errorf("path = %s", req.Path)
	}
}

// TestBatchRemoveGroupMembersMatchesDocumentedRequest covers the batch removal
// and the report of which blacklist additions failed.
func TestBatchRemoveGroupMembersMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"remove_members_result": "success",
		"add_to_member_blacklist_fail_openids": ["M2"]
	}`, func(c *Client) error {
		result, err := c.BatchRemoveGroupMembers(t.Context(), "G1", &BatchRemoveMembersRequest{
			MemberOpenIDs:        []string{"M1", "M2"},
			AddToMemberBlacklist: true,
		})
		if err != nil {
			return err
		}
		if result.Result != "success" || len(result.BlacklistFailedOpenIDs) != 1 {
			t.Errorf("result = %+v", result)
		}
		return nil
	})
	if req.Path != "/v2/groups/G1/batch_remove_members" {
		t.Errorf("path = %s", req.Path)
	}
	if decodeBody(t, req)["add_to_member_blacklist"] != true {
		t.Errorf("body = %v", decodeBody(t, req))
	}
}

// TestListGroupBlacklistMatchesDocumentedRequest covers the blacklist query.
func TestListGroupBlacklistMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"users": [{"member_openid":"M1","username":"小明","banned_at":"2026-07-21T10:00:00+08:00"}],
		"next_cursor": ""
	}`, func(c *Client) error {
		page, err := c.ListGroupBlacklist(t.Context(), "G1", "", 500)
		if err != nil {
			return err
		}
		if len(page.Users) != 1 || page.Users[0].Username != "小明" {
			t.Errorf("page = %+v", page)
		}
		return nil
	})
	if req.Path != "/v2/groups/G1/member_blacklist" {
		t.Errorf("path = %s", req.Path)
	}
	if req.Query != "limit=100" {
		t.Errorf("query = %q, want the limit capped at 100", req.Query)
	}
}

// TestUpdateGroupBlacklistMatchesDocumentedRequest covers the blacklist change
// and its failure report.
func TestUpdateGroupBlacklistMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{"fail_openids":["M2"]}`, func(c *Client) error {
		result, err := c.UpdateGroupBlacklist(t.Context(), "G1", &UpdateBlacklistRequest{
			Op:            ListOpAdd,
			MemberOpenIDs: []string{"M1", "M2"},
		})
		if err != nil {
			return err
		}
		if len(result.FailOpenIDs) != 1 || result.FailOpenIDs[0] != "M2" {
			t.Errorf("result = %+v", result)
		}
		return nil
	})
	if req.Method != http.MethodPost || req.Path != "/v2/groups/G1/member_blacklist" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
}

// TestListJoinApprovalStrategiesMatchesDocumentedRequest covers the strategy
// list.
func TestListJoinApprovalStrategiesMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{
		"strategies": [{
			"strategy_id": "st_1", "group_openids": ["G1"], "group_ids": [123],
			"whitelist_user_count": 3, "is_enable": "on",
			"expire_at": "2027-07-21T10:00:00+08:00", "remark": "生产测试"
		}],
		"next_cursor": "NEXT"
	}`, func(c *Client) error {
		page, err := c.ListJoinApprovalStrategies(t.Context(), "CUR", 999)
		if err != nil {
			return err
		}
		if len(page.Strategies) != 1 || page.NextCursor != "NEXT" {
			t.Fatalf("page = %+v", page)
		}
		strategy := page.Strategies[0]
		if strategy.StrategyID != "st_1" || strategy.IsEnable != StrategyEnabled {
			t.Errorf("strategy = %+v", strategy)
		}
		if len(strategy.GroupIDs) != 1 || strategy.GroupIDs[0] != 123 {
			t.Errorf("GroupIDs = %v", strategy.GroupIDs)
		}
		return nil
	})
	if req.Path != "/v2/groups/join_approval_strategy" {
		t.Errorf("path = %s", req.Path)
	}
	if !strings.Contains(req.Query, "limit=50") {
		t.Errorf("query = %q, want the limit capped at 50", req.Query)
	}
}

// TestCreateJoinApprovalStrategyMatchesDocumentedRequest covers strategy
// creation with the openid form.
func TestCreateJoinApprovalStrategyMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{"strategy_id":"st_new","is_enable":"on","expire_at":"2027-01-01T00:00:00+08:00"}`,
		func(c *Client) error {
			created, err := c.CreateJoinApprovalStrategy(t.Context(), &CreateJoinApprovalStrategyRequest{
				GroupOpenIDs: []string{"G1", "G2"},
				IsEnable:     StrategyEnabled,
				Remark:       "生产测试",
			})
			if err != nil {
				return err
			}
			if created.StrategyID != "st_new" {
				t.Errorf("created = %+v", created)
			}
			return nil
		})
	if req.Method != http.MethodPost || req.Path != "/v2/groups/join_approval_strategy" {
		t.Errorf("%s %s", req.Method, req.Path)
	}
	body := decodeBody(t, req)
	groups, ok := body["group_openids"].([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("group_openids = %v", body["group_openids"])
	}
	if _, present := body["group_ids"]; present {
		t.Errorf("body = %v, want only one group form", body)
	}
}

// TestUpdateJoinApprovalStrategyMatchesDocumentedRequest covers the patch,
// including the nested group action.
func TestUpdateJoinApprovalStrategyMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{"is_enable":"off","expire_at":"2027-01-01T00:00:00+08:00"}`,
		func(c *Client) error {
			updated, err := c.UpdateJoinApprovalStrategy(t.Context(), "st_1",
				&UpdateJoinApprovalStrategyRequest{
					IsEnable: StrategyDisabled,
					GroupAction: &StrategyGroupAction{
						Op:           StrategyGroupAdd,
						GroupOpenIDs: []string{"G3"},
					},
				})
			if err != nil {
				return err
			}
			if updated.IsEnable != StrategyDisabled {
				t.Errorf("updated = %+v", updated)
			}
			return nil
		})
	if req.Method != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", req.Method)
	}
	if req.Path != "/v2/groups/join_approval_strategy/st_1" {
		t.Errorf("path = %s", req.Path)
	}
	action, ok := decodeBody(t, req)["group_action"].(map[string]any)
	if !ok || action["op"] != StrategyGroupAdd {
		t.Errorf("body = %v", decodeBody(t, req))
	}
}

// TestDeleteAndExecuteJoinApprovalStrategy cover the two verb only calls.
func TestDeleteAndExecuteJoinApprovalStrategy(t *testing.T) {
	req := groupRequest(t, `{}`, func(c *Client) error {
		return c.DeleteJoinApprovalStrategy(t.Context(), "st_1")
	})
	if req.Method != http.MethodDelete || req.Path != "/v2/groups/join_approval_strategy/st_1" {
		t.Errorf("%s %s", req.Method, req.Path)
	}

	req = groupRequest(t, `{}`, func(c *Client) error {
		return c.ExecuteJoinApprovalStrategy(t.Context(), "st_1")
	})
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/v2/groups/join_approval_strategy/st_1/execute" {
		t.Errorf("path = %s", req.Path)
	}
}

// TestUpdateJoinApprovalStrategyWhitelistMatchesDocumentedRequest covers the
// whitelist call.
func TestUpdateJoinApprovalStrategyWhitelistMatchesDocumentedRequest(t *testing.T) {
	req := groupRequest(t, `{"strategy_id":"st_1","whitelist_user_count":2}`,
		func(c *Client) error {
			result, err := c.UpdateJoinApprovalStrategyWhitelist(t.Context(), "st_1",
				&UpdateWhitelistRequest{
					Op:             ListOpAdd,
					WhitelistUsers: []string{"10001", "10002"},
				})
			if err != nil {
				return err
			}
			if result.WhitelistUserCount != 2 {
				t.Errorf("result = %+v", result)
			}
			return nil
		})
	if req.Path != "/v2/groups/join_approval_strategy/st_1/whitelist_users" {
		t.Errorf("path = %s", req.Path)
	}
	users, ok := decodeBody(t, req)["whitelist_users"].([]any)
	if !ok || len(users) != 2 {
		t.Fatalf("whitelist_users = %v", decodeBody(t, req)["whitelist_users"])
	}
}

// TestGroupEndpointValidation covers the guards that keep a mistake from
// costing a round trip.
func TestGroupEndpointValidation(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	tooManyMembers := make([]string, MaxBatchRemoveMembers+1)
	tooManyMutes := make([]SetMemberMuteState, MaxGroupMuteTargets+1)
	tooManyNumbers := make([]string, MaxWhitelistUsers+1)
	tooManyGroups := make([]string, MaxStrategyGroups+1)

	cases := map[string]func() error{
		"info without a group":   func() error { _, err := client.GetGroupInfo(t.Context(), ""); return err },
		"state without a group":  func() error { _, err := client.GetGroupBotState(t.Context(), ""); return err },
		"requests without group": func() error { _, err := client.ListGroupJoinRequests(t.Context(), "", "", 0); return err },
		"approval without group": func() error {
			return client.ApproveGroupJoinRequest(t.Context(), "", "M1", &JoinRequestApproval{Op: JoinApprovalApprove})
		},
		"approval without member": func() error {
			return client.ApproveGroupJoinRequest(t.Context(), "G1", "", &JoinRequestApproval{Op: JoinApprovalApprove})
		},
		"approval without a request": func() error {
			return client.ApproveGroupJoinRequest(t.Context(), "G1", "M1", nil)
		},
		"approval with no op": func() error {
			return client.ApproveGroupJoinRequest(t.Context(), "G1", "M1", &JoinRequestApproval{})
		},
		"approval with a bad op": func() error {
			return client.ApproveGroupJoinRequest(t.Context(), "G1", "M1", &JoinRequestApproval{Op: "maybe"})
		},
		"mute without members": func() error {
			return client.SetGroupMemberMute(t.Context(), "G1", &SetGroupMemberMuteRequest{})
		},
		"mute over the limit": func() error {
			return client.SetGroupMemberMute(t.Context(), "G1",
				&SetGroupMemberMuteRequest{Members: tooManyMutes})
		},
		"mute with a bad op": func() error {
			return client.SetGroupMemberMute(t.Context(), "G1", &SetGroupMemberMuteRequest{
				Members: []SetMemberMuteState{{Op: "silence", MemberOpenID: "M1"}},
			})
		},
		"mute without a member": func() error {
			return client.SetGroupMemberMute(t.Context(), "G1", &SetGroupMemberMuteRequest{
				Members: []SetMemberMuteState{{Op: MemberMuteAdd}},
			})
		},
		"member without a group": func() error {
			_, err := client.ListGroupMembers(t.Context(), "", "")
			return err
		},
		"single member without a member openid": func() error {
			_, err := client.GetGroupMember(t.Context(), "G1", "")
			return err
		},
		"removal without members": func() error {
			_, err := client.BatchRemoveGroupMembers(t.Context(), "G1", &BatchRemoveMembersRequest{})
			return err
		},
		"removal over the limit": func() error {
			_, err := client.BatchRemoveGroupMembers(t.Context(), "G1",
				&BatchRemoveMembersRequest{MemberOpenIDs: tooManyMembers})
			return err
		},
		"blacklist without a group": func() error {
			_, err := client.ListGroupBlacklist(t.Context(), "", "", 0)
			return err
		},
		"blacklist change without a request": func() error {
			_, err := client.UpdateGroupBlacklist(t.Context(), "G1", nil)
			return err
		},
		"blacklist change with no op": func() error {
			_, err := client.UpdateGroupBlacklist(t.Context(), "G1",
				&UpdateBlacklistRequest{MemberOpenIDs: []string{"M1"}})
			return err
		},
		"blacklist change with a bad op": func() error {
			_, err := client.UpdateGroupBlacklist(t.Context(), "G1",
				&UpdateBlacklistRequest{Op: "clear", MemberOpenIDs: []string{"M1"}})
			return err
		},
		"blacklist change without members": func() error {
			_, err := client.UpdateGroupBlacklist(t.Context(), "G1", &UpdateBlacklistRequest{Op: ListOpAdd})
			return err
		},
		"blacklist change over the limit": func() error {
			_, err := client.UpdateGroupBlacklist(t.Context(), "G1",
				&UpdateBlacklistRequest{Op: ListOpAdd, MemberOpenIDs: tooManyMembers})
			return err
		},
		"strategy without a request": func() error {
			_, err := client.CreateJoinApprovalStrategy(t.Context(), nil)
			return err
		},
		"strategy with neither group form": func() error {
			_, err := client.CreateJoinApprovalStrategy(t.Context(),
				&CreateJoinApprovalStrategyRequest{})
			return err
		},
		"strategy with both group forms": func() error {
			_, err := client.CreateJoinApprovalStrategy(t.Context(), &CreateJoinApprovalStrategyRequest{
				GroupOpenIDs: []string{"G1"}, GroupIDs: []uint64{123},
			})
			return err
		},
		"strategy over the group limit": func() error {
			_, err := client.CreateJoinApprovalStrategy(t.Context(),
				&CreateJoinApprovalStrategyRequest{GroupOpenIDs: tooManyGroups})
			return err
		},
		"strategy with a bad enable value": func() error {
			_, err := client.CreateJoinApprovalStrategy(t.Context(), &CreateJoinApprovalStrategyRequest{
				GroupOpenIDs: []string{"G1"}, IsEnable: "yes",
			})
			return err
		},
		"update without a strategy id": func() error {
			_, err := client.UpdateJoinApprovalStrategy(t.Context(), "",
				&UpdateJoinApprovalStrategyRequest{IsEnable: StrategyEnabled})
			return err
		},
		"update with a bad enable value": func() error {
			_, err := client.UpdateJoinApprovalStrategy(t.Context(), "st_1",
				&UpdateJoinApprovalStrategyRequest{IsEnable: "yes"})
			return err
		},
		"update with a bad group action op": func() error {
			_, err := client.UpdateJoinApprovalStrategy(t.Context(), "st_1",
				&UpdateJoinApprovalStrategyRequest{GroupAction: &StrategyGroupAction{
					Op: "keep", GroupOpenIDs: []string{"G1"},
				}})
			return err
		},
		"update with both group forms": func() error {
			_, err := client.UpdateJoinApprovalStrategy(t.Context(), "st_1",
				&UpdateJoinApprovalStrategyRequest{GroupAction: &StrategyGroupAction{
					Op: StrategyGroupAdd, GroupOpenIDs: []string{"G1"}, GroupIDs: []uint64{1},
				}})
			return err
		},
		"delete without a strategy id": func() error {
			return client.DeleteJoinApprovalStrategy(t.Context(), "")
		},
		"execute without a strategy id": func() error {
			return client.ExecuteJoinApprovalStrategy(t.Context(), "")
		},
		"whitelist without a strategy id": func() error {
			_, err := client.UpdateJoinApprovalStrategyWhitelist(t.Context(), "",
				&UpdateWhitelistRequest{Op: ListOpAdd, WhitelistUsers: []string{"1"}})
			return err
		},
		"whitelist without numbers": func() error {
			_, err := client.UpdateJoinApprovalStrategyWhitelist(t.Context(), "st_1",
				&UpdateWhitelistRequest{Op: ListOpAdd})
			return err
		},
		"whitelist with a bad op": func() error {
			_, err := client.UpdateJoinApprovalStrategyWhitelist(t.Context(), "st_1",
				&UpdateWhitelistRequest{Op: "set", WhitelistUsers: []string{"1"}})
			return err
		},
		"whitelist over the limit": func() error {
			_, err := client.UpdateJoinApprovalStrategyWhitelist(t.Context(), "st_1",
				&UpdateWhitelistRequest{Op: ListOpAdd, WhitelistUsers: tooManyNumbers})
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("expected a validation error")
			}
		})
	}

	if len(*captured) != 0 {
		t.Errorf("a rejected call still sent %d requests", len(*captured))
	}
}

// TestGroupEndpointsEscapeThePath checks that identifiers cannot break a route.
func TestGroupEndpointsEscapeThePath(t *testing.T) {
	req := groupRequest(t, `{}`, func(c *Client) error {
		return c.DeleteJoinApprovalStrategy(t.Context(), "st/1?x=2")
	})
	if !strings.Contains(req.EscapedPath, "%2F") {
		t.Errorf("escaped path = %s, want the slash escaped", req.EscapedPath)
	}
	if req.Query != "" {
		t.Errorf("query = %q, want the question mark escaped", req.Query)
	}
}

// TestGroupEndpointsPropagateErrors checks that a platform failure surfaces.
func TestGroupEndpointsPropagateErrors(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"err_code":11251,"message":"appid 错误"}`)

	if _, err := client.GetGroupInfo(t.Context(), "G1"); err == nil {
		t.Error("GetGroupInfo must surface a platform failure")
	}
	if _, err := client.ListGroupMembers(t.Context(), "G1", ""); err == nil {
		t.Error("ListGroupMembers must surface a platform failure")
	}
}
