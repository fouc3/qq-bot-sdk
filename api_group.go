package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Paths and documented limits of the group management endpoints.
const (
	groupPathPrefix = "/v2/groups/"

	// maxGroupMembersPerPage is the largest page the member list returns.
	maxGroupMembersPerPage = 30
	// DefaultGroupListPageSize is the page size used when none is requested.
	DefaultGroupListPageSize = 20
	// MaxJoinRequestPageSize is the documented maximum for a join request page.
	MaxJoinRequestPageSize = 50
	// MaxBlacklistPageSize is the documented maximum for a blacklist page.
	MaxBlacklistPageSize = 100
	// MaxStrategyPageSize is the documented maximum for a strategy page.
	MaxStrategyPageSize = 50
	// MaxGroupMuteTargets is how many members one mute request may change.
	MaxGroupMuteTargets = 20
	// MaxBatchRemoveMembers is how many members one removal may target.
	MaxBatchRemoveMembers = 20
	// MaxBlacklistTargets is how many members one blacklist call may change.
	MaxBlacklistTargets = 20
	// MaxStrategyGroups is how many groups one strategy may cover.
	MaxStrategyGroups = 100
	// MaxWhitelistUsers is how many numbers one whitelist call may change.
	MaxWhitelistUsers = 10000
	// strategyPath is the automatic join approval strategy collection.
	strategyPath = "/v2/groups/join_approval_strategy"
)

// Incoming message settings reported by GetGroupBotState.
const (
	// GroupRecvMsgAll receives every group message.
	GroupRecvMsgAll = "all"
	// GroupRecvMsgOnlyMention receives only mentions.
	GroupRecvMsgOnlyMention = "only_mention"
	// GroupRecvMsgMentionAndContext receives mentions and their context.
	GroupRecvMsgMentionAndContext = "mention_and_context"
)

// Group member roles.
const (
	// GroupRoleMember is an ordinary member.
	GroupRoleMember = "member"
	// GroupRoleOwner is the group owner.
	GroupRoleOwner = "owner"
	// GroupRoleAdmin is an administrator.
	GroupRoleAdmin = "admin"
)

// Join request approval actions.
const (
	// JoinApprovalApprove accepts the request.
	JoinApprovalApprove = "approve"
	// JoinApprovalDecline rejects the request.
	JoinApprovalDecline = "decline"
)

// Member mute operations.
const (
	// MemberMuteAdd adds a mute.
	MemberMuteAdd = "add"
	// MemberMuteUpdate changes when an existing mute expires.
	MemberMuteUpdate = "update"
	// MemberMuteDelete lifts a mute.
	MemberMuteDelete = "del"
)

// Whole group mute modes.
const (
	// GroupMuteNone means the group is not muted.
	GroupMuteNone = "none"
	// GroupMuteAlways means the group is always muted.
	GroupMuteAlways = "always"
	// GroupMuteSchedule means the group is muted on a schedule.
	GroupMuteSchedule = "schedule"
)

// Strategy enable states.
const (
	// StrategyEnabled turns a strategy on.
	StrategyEnabled = "on"
	// StrategyDisabled turns a strategy off.
	StrategyDisabled = "off"
)

// Strategy group association operations.
const (
	// StrategyGroupAdd associates more groups.
	StrategyGroupAdd = "add"
	// StrategyGroupDelete dissociates groups.
	StrategyGroupDelete = "del"
)

// Blacklist and whitelist operations.
const (
	// ListOpAdd adds to the list.
	ListOpAdd = "add"
	// ListOpDelete removes from the list.
	ListOpDelete = "del"
)

// GroupInfo is the basic information of one group.
type GroupInfo struct {
	// GroupOpenID is the group OpenID.
	GroupOpenID string `json:"group_openid"`
	// GroupName is the group name.
	GroupName string `json:"group_name,omitempty"`
	// GroupFingerMemo is the group description.
	GroupFingerMemo string `json:"group_finger_memo,omitempty"`
	// GroupClassText is the group category.
	GroupClassText string `json:"group_class_text,omitempty"`
	// GroupTags are the group tags.
	GroupTags []string `json:"group_tags,omitempty"`
	// GroupMemberNum is the member count.
	GroupMemberNum int `json:"group_member_num,omitempty"`
}

// GroupBotState is the bot's own standing in a group.
type GroupBotState struct {
	// MemberOpenID is the bot's openid in this group.
	MemberOpenID string `json:"member_openid,omitempty"`
	// JoinedAt is when the bot joined, in RFC3339.
	JoinedAt string `json:"joined_at,omitempty"`
	// AllowProactiveMsg reports whether the group accepts proactive pushes.
	AllowProactiveMsg bool `json:"allow_proactive_msg,omitempty"`
	// RecvMsgSetting is one of the GroupRecvMsg constants.
	RecvMsgSetting string `json:"recv_msg_setting,omitempty"`
	// MemberRole is the bot's role: member, owner or admin.
	MemberRole string `json:"member_role,omitempty"`
}

// JoinRequest is one pending request to join a group.
type JoinRequest struct {
	// JoinRequestID must be echoed when approving.
	JoinRequestID string `json:"join_request_id,omitempty"`
	// RiskTips is a safety hint; warning_tips marks a suspicious request and
	// top_tips a request that matched sec_risk_rules.
	RiskTips string `json:"risk_tips,omitempty"`
	// UnionOpenID is the cross-application identity, when available.
	UnionOpenID string `json:"union_openid,omitempty"`
	// MemberOpenID identifies the applicant.
	MemberOpenID string `json:"member_openid,omitempty"`
	// Username is the applicant's nickname.
	Username string `json:"username,omitempty"`
	// ApplyAt is the request time in RFC3339.
	ApplyAt string `json:"apply_at,omitempty"`
	// ApplySource is GroupJoinSourceSelf or GroupJoinSourceInvited.
	ApplySource string `json:"apply_source,omitempty"`
	// InvitedBy is the inviter when ApplySource is an invitation.
	InvitedBy string `json:"invited_by,omitempty"`
	// Bot reports whether the applicant is a bot account.
	Bot bool `json:"bot,omitempty"`
	// VerifyInfo describes how the applicant verified itself.
	VerifyInfo *VerifyInfo `json:"verify_info,omitempty"`
}

// JoinRequestList is one page of join requests.
type JoinRequestList struct {
	// List holds the requests.
	List []JoinRequest `json:"list,omitempty"`
	// NextCursor fetches the next page; empty means the end.
	NextCursor string `json:"next_cursor,omitempty"`
}

// JoinRequestApproval answers one join request.
type JoinRequestApproval struct {
	// Op is JoinApprovalApprove or JoinApprovalDecline, and is required.
	Op string `json:"op"`
	// JoinRequestID is the id from the request, and should be echoed.
	JoinRequestID string `json:"join_request_id,omitempty"`
	// RejectReason explains a decline.
	RejectReason string `json:"reject_reason,omitempty"`
	// AddToMemberBlacklist also blacklists the applicant, which is only
	// meaningful when declining.
	AddToMemberBlacklist bool `json:"add_to_member_blacklist,omitempty"`
}

// MuteScheduleRule is one timed whole group mute.
type MuteScheduleRule struct {
	// TaskID marks the task.
	TaskID string `json:"task_id,omitempty"`
	// StartAt is when the mute starts, in RFC3339.
	StartAt string `json:"start_at,omitempty"`
	// EndAt is when it ends, in RFC3339.
	EndAt string `json:"end_at,omitempty"`
	// Enabled reports whether the rule applies.
	Enabled bool `json:"enabled,omitempty"`
}

// MuteRecurringRule is one weekly whole group mute.
type MuteRecurringRule struct {
	// TaskID marks the task.
	TaskID string `json:"task_id,omitempty"`
	// Weekdays are the days it applies, 1 is Monday and 7 is Sunday.
	Weekdays []int `json:"weekdays,omitempty"`
	// StartTime is the daily start, HH:mm in Beijing time.
	StartTime string `json:"start_time,omitempty"`
	// EndTime is the daily end; a value before StartTime crosses midnight.
	EndTime string `json:"end_time,omitempty"`
	// Enabled reports whether the rule applies.
	Enabled bool `json:"enabled,omitempty"`
}

// GroupMuteRule is the whole group mute configuration.
type GroupMuteRule struct {
	// Mode is one of the GroupMute constants.
	Mode string `json:"mode,omitempty"`
	// ScheduleRules are the timed mutes.
	ScheduleRules []MuteScheduleRule `json:"schedule_rules,omitempty"`
	// RecurringRules are the weekly mutes.
	RecurringRules []MuteRecurringRule `json:"recurring_rules,omitempty"`
}

// MemberMuteState is one currently muted member.
type MemberMuteState struct {
	// MemberOpenID identifies the muted member.
	MemberOpenID string `json:"member_openid,omitempty"`
	// MuteExpireAt is when the mute ends, in RFC3339.
	MuteExpireAt string `json:"mute_expire_at,omitempty"`
	// Username is the member's nickname.
	Username string `json:"username,omitempty"`
	// UnionOpenID is the cross-application identity, when available.
	UnionOpenID string `json:"union_openid,omitempty"`
}

// GroupRestrictChatSetting is the mute state of one group.
type GroupRestrictChatSetting struct {
	// GlobalRule is the whole group mute configuration.
	GlobalRule *GroupMuteRule `json:"global_rule,omitempty"`
	// Members are the members currently muted, expired ones excluded.
	Members []MemberMuteState `json:"members,omitempty"`
}

// SetMemberMuteState changes one member's mute.
type SetMemberMuteState struct {
	// Op is one of the MemberMute constants, and is required.
	Op string `json:"op"`
	// MemberOpenID identifies the member, and is required. Only an ordinary
	// member may be muted, not the owner, an administrator or a bot.
	MemberOpenID string `json:"member_openid"`
	// MuteExpireAt is when the mute ends, in RFC3339. For a delete an empty
	// string lifts the mute immediately.
	MuteExpireAt string `json:"mute_expire_at,omitempty"`
}

// SetGroupMemberMuteRequest changes up to MaxGroupMuteTargets mutes at once.
type SetGroupMemberMuteRequest struct {
	// Members are the changes, at most 20.
	Members []SetMemberMuteState `json:"members,omitempty"`
}

// GroupMember is one member of a group.
type GroupMember struct {
	// MemberOpenID identifies the member.
	MemberOpenID string `json:"member_openid,omitempty"`
	// Username is the member's nickname.
	Username string `json:"username,omitempty"`
	// MemberRole is member, owner or admin.
	MemberRole string `json:"member_role,omitempty"`
	// Bot reports whether the member is a bot.
	Bot bool `json:"bot,omitempty"`
	// JoinedAt is when they joined, in RFC3339.
	JoinedAt string `json:"joined_at,omitempty"`
	// UnionOpenID is the cross-application identity, when available.
	UnionOpenID string `json:"union_openid,omitempty"`
}

// GroupMemberList is one page of members.
type GroupMemberList struct {
	// Members is the page, at most MaxGroupMembersPerPage entries.
	Members []GroupMember `json:"members,omitempty"`
	// NextCursor fetches the next page; empty means the end.
	NextCursor string `json:"next_cursor,omitempty"`
}

// BatchRemoveMembersRequest removes several members at once.
type BatchRemoveMembersRequest struct {
	// MemberOpenIDs are the members to remove, at most 20, and are required.
	MemberOpenIDs []string `json:"member_openids"`
	// AddToMemberBlacklist also blacklists them, defaulting to false.
	AddToMemberBlacklist bool `json:"add_to_member_blacklist,omitempty"`
}

// BatchRemoveMembersResult reports which blacklist additions failed.
type BatchRemoveMembersResult struct {
	// Result is "success" when the removal worked.
	Result string `json:"remove_members_result,omitempty"`
	// BlacklistFailedOpenIDs are the members that could not be blacklisted.
	BlacklistFailedOpenIDs []string `json:"add_to_member_blacklist_fail_openids,omitempty"`
}

// BlacklistUser is one blacklisted user.
type BlacklistUser struct {
	// UnionOpenID is the cross-application identity, when available.
	UnionOpenID string `json:"union_openid,omitempty"`
	// MemberOpenID identifies the user.
	MemberOpenID string `json:"member_openid,omitempty"`
	// Username is the user's nickname.
	Username string `json:"username,omitempty"`
	// BannedAt is when they were blacklisted, in RFC3339.
	BannedAt string `json:"banned_at,omitempty"`
}

// GroupBlacklist is one page of blacklisted users.
type GroupBlacklist struct {
	// Users is the page.
	Users []BlacklistUser `json:"users,omitempty"`
	// NextCursor fetches the next page; empty means the end.
	NextCursor string `json:"next_cursor,omitempty"`
}

// UpdateBlacklistRequest changes the group blacklist.
type UpdateBlacklistRequest struct {
	// Op is ListOpAdd or ListOpDelete, and is required. A member who is still
	// in the group cannot be blacklisted.
	Op string `json:"op"`
	// MemberOpenIDs are the targets, at most 20, and are required.
	MemberOpenIDs []string `json:"member_openids"`
}

// UpdateBlacklistResult reports which changes failed.
type UpdateBlacklistResult struct {
	// FailOpenIDs are the members the operation could not be applied to.
	FailOpenIDs []string `json:"fail_openids,omitempty"`
}

// JoinApprovalStrategy is one automatic join approval strategy.
type JoinApprovalStrategy struct {
	// StrategyID identifies the strategy.
	StrategyID string `json:"strategy_id,omitempty"`
	// GroupOpenIDs are the associated groups, returned when the strategy was
	// created with openids.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	// GroupIDs are the associated QQ group numbers, returned when the strategy
	// was created with numbers.
	GroupIDs []uint64 `json:"group_ids,omitempty"`
	// WhitelistUserCount is roughly how many numbers are whitelisted.
	WhitelistUserCount int `json:"whitelist_user_count,omitempty"`
	// IsEnable is StrategyEnabled or StrategyDisabled.
	IsEnable string `json:"is_enable,omitempty"`
	// ExpireAt is when the strategy stops applying, in RFC3339.
	ExpireAt string `json:"expire_at,omitempty"`
	// CreatedAt is the creation time, in RFC3339.
	CreatedAt string `json:"created_at,omitempty"`
	// UpdatedAt is the last change, in RFC3339.
	UpdatedAt string `json:"updated_at,omitempty"`
	// Remark is the developer note.
	Remark string `json:"remark,omitempty"`
}

// JoinApprovalStrategyList is one page of strategies.
type JoinApprovalStrategyList struct {
	// Strategies are the active strategies.
	Strategies []JoinApprovalStrategy `json:"strategies,omitempty"`
	// NextCursor fetches the next page; empty means the end.
	NextCursor string `json:"next_cursor,omitempty"`
}

// CreateJoinApprovalStrategyRequest creates a strategy.
//
// Exactly one of GroupOpenIDs and GroupIDs must be set: passing both, or
// neither, is an error.
type CreateJoinApprovalStrategyRequest struct {
	// GroupOpenIDs are the groups to cover, at most 100, mutually exclusive
	// with GroupIDs.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	// GroupIDs are QQ group numbers to cover, at most 100, mutually exclusive
	// with GroupOpenIDs.
	GroupIDs []uint64 `json:"group_ids,omitempty"`
	// IsEnable is StrategyEnabled or StrategyDisabled; the platform defaults
	// to enabled.
	IsEnable string `json:"is_enable,omitempty"`
	// ExpireAt is when the strategy stops applying, in RFC3339. The platform
	// defaults to one year.
	ExpireAt string `json:"expire_at,omitempty"`
	// Remark is the developer note, at most 255 characters.
	Remark string `json:"remark,omitempty"`
}

// CreatedJoinApprovalStrategy is the response to creating a strategy.
type CreatedJoinApprovalStrategy struct {
	// StrategyID is the id the platform assigned.
	StrategyID string `json:"strategy_id"`
	// IsEnable is the resulting state.
	IsEnable string `json:"is_enable,omitempty"`
	// ExpireAt is the resulting expiry, in RFC3339.
	ExpireAt string `json:"expire_at,omitempty"`
}

// StrategyGroupAction adds or removes the groups a strategy covers.
type StrategyGroupAction struct {
	// Op is StrategyGroupAdd or StrategyGroupDelete, and is required.
	Op string `json:"op"`
	// GroupOpenIDs are the groups, mutually exclusive with GroupIDs.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	// GroupIDs are QQ group numbers, mutually exclusive with GroupOpenIDs.
	GroupIDs []uint64 `json:"group_ids,omitempty"`
}

// UpdateJoinApprovalStrategyRequest changes a strategy.
type UpdateJoinApprovalStrategyRequest struct {
	// IsEnable is StrategyEnabled or StrategyDisabled.
	IsEnable string `json:"is_enable,omitempty"`
	// ExpireAt is the new expiry, in RFC3339.
	ExpireAt string `json:"expire_at,omitempty"`
	// GroupAction adds or removes covered groups. The group identifier form
	// must match the one the strategy was created with.
	GroupAction *StrategyGroupAction `json:"group_action,omitempty"`
	// Remark is the developer note, at most 255 characters.
	Remark string `json:"remark,omitempty"`
}

// UpdatedJoinApprovalStrategy is the response to changing a strategy.
type UpdatedJoinApprovalStrategy struct {
	// IsEnable is the resulting state.
	IsEnable string `json:"is_enable,omitempty"`
	// ExpireAt is the resulting expiry, in RFC3339.
	ExpireAt string `json:"expire_at,omitempty"`
}

// UpdateWhitelistRequest changes a strategy's whitelist.
type UpdateWhitelistRequest struct {
	// Op is ListOpAdd or ListOpDelete, and is required.
	Op string `json:"op"`
	// WhitelistUsers are QQ numbers sent as strings, at most 10000, and are
	// required. The documentation asks for strings to avoid a JavaScript
	// precision problem.
	WhitelistUsers []string `json:"whitelist_users"`
}

// WhitelistResult is the response to changing a whitelist.
type WhitelistResult struct {
	// StrategyID is the strategy changed.
	StrategyID string `json:"strategy_id,omitempty"`
	// WhitelistUserCount is roughly the resulting whitelist size.
	WhitelistUserCount int `json:"whitelist_user_count,omitempty"`
	// UpdatedAt is the change time, in RFC3339.
	UpdatedAt string `json:"updated_at,omitempty"`
}

// groupPath builds an address under one group.
func groupPath(groupOpenID, suffix string) string {
	return groupPathPrefix + url.PathEscape(groupOpenID) + "/" + suffix
}

// groupQuery builds a page query, capping limit at the documented maximum.
func groupQuery(cursor string, limit, maxSize int) string {
	query := url.Values{}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		if limit > maxSize {
			limit = maxSize
		}
		query.Set("limit", strconv.Itoa(limit))
	}
	if encoded := query.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

// requireGroupOpenID rejects a call without a group.
func requireGroupOpenID(groupOpenID, method string) error {
	if groupOpenID == "" {
		return fmt.Errorf("qqbotsdk: %s needs a group openid", method)
	}
	return nil
}

// GetGroupInfo returns the basic information of one group.
func (c *Client) GetGroupInfo(ctx context.Context, groupOpenID string) (*GroupInfo, error) {
	if err := requireGroupOpenID(groupOpenID, "GetGroupInfo"); err != nil {
		return nil, err
	}
	var out GroupInfo
	if err := c.doJSON(ctx, http.MethodGet, groupPath(groupOpenID, "info"), nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGroupBotState returns the bot's own standing in one group.
func (c *Client) GetGroupBotState(ctx context.Context, groupOpenID string) (*GroupBotState, error) {
	if err := requireGroupOpenID(groupOpenID, "GetGroupBotState"); err != nil {
		return nil, err
	}
	var out GroupBotState
	if err := c.doJSON(ctx, http.MethodGet, groupPath(groupOpenID, "bot_state"), nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGroupJoinRequests lists the pending requests to join one group.
//
// cursor is the NextCursor of the previous page, empty for the first. limit
// defaults to 20 and is capped at 50.
func (c *Client) ListGroupJoinRequests(ctx context.Context, groupOpenID, cursor string, limit int) (*JoinRequestList, error) {
	if err := requireGroupOpenID(groupOpenID, "ListGroupJoinRequests"); err != nil {
		return nil, err
	}
	path := groupPath(groupOpenID, "join_request_list") + groupQuery(cursor, limit, MaxJoinRequestPageSize)

	var out JoinRequestList
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApproveGroupJoinRequest answers one request to join a group.
//
// The bot must be a group administrator.
func (c *Client) ApproveGroupJoinRequest(ctx context.Context, groupOpenID, memberOpenID string, req *JoinRequestApproval) error {
	if err := requireGroupOpenID(groupOpenID, "ApproveGroupJoinRequest"); err != nil {
		return err
	}
	if memberOpenID == "" {
		return errors.New("qqbotsdk: ApproveGroupJoinRequest needs a member openid")
	}
	if req == nil {
		return errors.New("qqbotsdk: ApproveGroupJoinRequest needs a request")
	}
	switch req.Op {
	case JoinApprovalApprove, JoinApprovalDecline:
	case "":
		return errors.New("qqbotsdk: op is required, " + JoinApprovalApprove + " or " + JoinApprovalDecline)
	default:
		return fmt.Errorf("qqbotsdk: op %q is not %s or %s", req.Op, JoinApprovalApprove, JoinApprovalDecline)
	}

	path := groupPath(groupOpenID, "approval_join_request/"+url.PathEscape(memberOpenID))
	return c.doJSON(ctx, http.MethodPost, path, req, nil, openAPICall)
}

// GetGroupRestrictChatSetting returns the mute state of one group.
func (c *Client) GetGroupRestrictChatSetting(ctx context.Context, groupOpenID string) (*GroupRestrictChatSetting, error) {
	if err := requireGroupOpenID(groupOpenID, "GetGroupRestrictChatSetting"); err != nil {
		return nil, err
	}
	var out GroupRestrictChatSetting
	if err := c.doJSON(ctx, http.MethodGet, groupPath(groupOpenID, "restrict_chat_setting"), nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetGroupMemberMute changes the mutes of up to 20 members at once.
//
// The bot must be a group administrator, and a mute may last at most 30 days.
// Only an ordinary member can be muted, not the owner, another administrator or
// a bot.
func (c *Client) SetGroupMemberMute(ctx context.Context, groupOpenID string, req *SetGroupMemberMuteRequest) error {
	if err := requireGroupOpenID(groupOpenID, "SetGroupMemberMute"); err != nil {
		return err
	}
	if req == nil || len(req.Members) == 0 {
		return errors.New("qqbotsdk: SetGroupMemberMute needs at least one member")
	}
	if len(req.Members) > MaxGroupMuteTargets {
		return fmt.Errorf("qqbotsdk: SetGroupMemberMute takes at most %d members, got %d",
			MaxGroupMuteTargets, len(req.Members))
	}
	for i, member := range req.Members {
		if member.MemberOpenID == "" {
			return fmt.Errorf("qqbotsdk: member %d needs a member_openid", i)
		}
		switch member.Op {
		case MemberMuteAdd, MemberMuteUpdate, MemberMuteDelete:
		default:
			return fmt.Errorf("qqbotsdk: member %d op %q is not %s, %s or %s",
				i, member.Op, MemberMuteAdd, MemberMuteUpdate, MemberMuteDelete)
		}
	}
	return c.doJSON(ctx, http.MethodPost, groupPath(groupOpenID, "restrict_chat_setting"), req, nil, openAPICall)
}

// ListGroupMembers lists the members of one group, at most 30 per page.
//
// cursor is the NextCursor of the previous page, empty for the first.
func (c *Client) ListGroupMembers(ctx context.Context, groupOpenID, cursor string) (*GroupMemberList, error) {
	if err := requireGroupOpenID(groupOpenID, "ListGroupMembers"); err != nil {
		return nil, err
	}
	path := groupPath(groupOpenID, "members") + groupQuery(cursor, 0, 0)

	var out GroupMemberList
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetGroupMember returns one member of a group.
func (c *Client) GetGroupMember(ctx context.Context, groupOpenID, memberOpenID string) (*GroupMember, error) {
	if err := requireGroupOpenID(groupOpenID, "GetGroupMember"); err != nil {
		return nil, err
	}
	if memberOpenID == "" {
		return nil, errors.New("qqbotsdk: GetGroupMember needs a member openid")
	}
	var out GroupMember
	path := groupPath(groupOpenID, "members/"+url.PathEscape(memberOpenID))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// BatchRemoveGroupMembers removes up to 20 members from one group.
//
// It reports which blacklist additions failed, when one was asked for.
func (c *Client) BatchRemoveGroupMembers(ctx context.Context, groupOpenID string, req *BatchRemoveMembersRequest) (*BatchRemoveMembersResult, error) {
	if err := requireGroupOpenID(groupOpenID, "BatchRemoveGroupMembers"); err != nil {
		return nil, err
	}
	if req == nil || len(req.MemberOpenIDs) == 0 {
		return nil, errors.New("qqbotsdk: BatchRemoveGroupMembers needs at least one member openid")
	}
	if len(req.MemberOpenIDs) > MaxBatchRemoveMembers {
		return nil, fmt.Errorf("qqbotsdk: BatchRemoveGroupMembers takes at most %d members, got %d",
			MaxBatchRemoveMembers, len(req.MemberOpenIDs))
	}

	var out BatchRemoveMembersResult
	if err := c.doJSON(ctx, http.MethodPost, groupPath(groupOpenID, "batch_remove_members"), req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListGroupBlacklist lists the blacklisted users of one group.
//
// cursor is the NextCursor of the previous page, empty for the first. limit
// defaults to 20 and is capped at 100.
func (c *Client) ListGroupBlacklist(ctx context.Context, groupOpenID, cursor string, limit int) (*GroupBlacklist, error) {
	if err := requireGroupOpenID(groupOpenID, "ListGroupBlacklist"); err != nil {
		return nil, err
	}
	path := groupPath(groupOpenID, "member_blacklist") + groupQuery(cursor, limit, MaxBlacklistPageSize)

	var out GroupBlacklist
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateGroupBlacklist adds up to 20 members to the blacklist, or removes them.
//
// A member who is still in the group cannot be added.
func (c *Client) UpdateGroupBlacklist(ctx context.Context, groupOpenID string, req *UpdateBlacklistRequest) (*UpdateBlacklistResult, error) {
	if err := requireGroupOpenID(groupOpenID, "UpdateGroupBlacklist"); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errors.New("qqbotsdk: UpdateGroupBlacklist needs a request")
	}
	switch req.Op {
	case ListOpAdd, ListOpDelete:
	case "":
		return nil, errors.New("qqbotsdk: op is required, " + ListOpAdd + " or " + ListOpDelete)
	default:
		return nil, fmt.Errorf("qqbotsdk: op %q is not %s or %s", req.Op, ListOpAdd, ListOpDelete)
	}
	if len(req.MemberOpenIDs) == 0 {
		return nil, errors.New("qqbotsdk: UpdateGroupBlacklist needs at least one member openid")
	}
	if len(req.MemberOpenIDs) > MaxBlacklistTargets {
		return nil, fmt.Errorf("qqbotsdk: UpdateGroupBlacklist takes at most %d members, got %d",
			MaxBlacklistTargets, len(req.MemberOpenIDs))
	}

	var out UpdateBlacklistResult
	if err := c.doJSON(ctx, http.MethodPost, groupPath(groupOpenID, "member_blacklist"), req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListJoinApprovalStrategies lists the active automatic join approval
// strategies.
//
// cursor is the NextCursor of the previous page, empty for the first. limit
// defaults to 20 and is capped at 50.
func (c *Client) ListJoinApprovalStrategies(ctx context.Context, cursor string, limit int) (*JoinApprovalStrategyList, error) {
	path := strategyPath + groupQuery(cursor, limit, MaxStrategyPageSize)

	var out JoinApprovalStrategyList
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateJoinApprovalStrategy creates an automatic join approval strategy.
//
// Exactly one of GroupOpenIDs and GroupIDs must be set: the platform refuses
// both together and neither, so that mistake is caught here.
func (c *Client) CreateJoinApprovalStrategy(ctx context.Context, req *CreateJoinApprovalStrategyRequest) (*CreatedJoinApprovalStrategy, error) {
	if req == nil {
		return nil, errors.New("qqbotsdk: CreateJoinApprovalStrategy needs a request")
	}
	if err := validateOneGroupForm("CreateJoinApprovalStrategy",
		len(req.GroupOpenIDs), len(req.GroupIDs), MaxStrategyGroups); err != nil {
		return nil, err
	}
	switch req.IsEnable {
	case "", StrategyEnabled, StrategyDisabled:
	default:
		return nil, fmt.Errorf("qqbotsdk: is_enable %q is not %s or %s, or empty",
			req.IsEnable, StrategyEnabled, StrategyDisabled)
	}

	var out CreatedJoinApprovalStrategy
	if err := c.doJSON(ctx, http.MethodPost, strategyPath, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateJoinApprovalStrategy changes one strategy.
func (c *Client) UpdateJoinApprovalStrategy(ctx context.Context, strategyID string, req *UpdateJoinApprovalStrategyRequest) (*UpdatedJoinApprovalStrategy, error) {
	if strategyID == "" {
		return nil, errors.New("qqbotsdk: UpdateJoinApprovalStrategy needs a strategy id")
	}
	if req == nil {
		return nil, errors.New("qqbotsdk: UpdateJoinApprovalStrategy needs a request")
	}
	switch req.IsEnable {
	case "", StrategyEnabled, StrategyDisabled:
	default:
		return nil, fmt.Errorf("qqbotsdk: is_enable %q is not %s or %s, or empty",
			req.IsEnable, StrategyEnabled, StrategyDisabled)
	}
	if req.GroupAction != nil {
		switch req.GroupAction.Op {
		case StrategyGroupAdd, StrategyGroupDelete:
		default:
			return nil, fmt.Errorf("qqbotsdk: group_action op %q is not %s or %s",
				req.GroupAction.Op, StrategyGroupAdd, StrategyGroupDelete)
		}
		if err := validateOneGroupForm("UpdateJoinApprovalStrategy group_action",
			len(req.GroupAction.GroupOpenIDs), len(req.GroupAction.GroupIDs), MaxStrategyGroups); err != nil {
			return nil, err
		}
	}

	var out UpdatedJoinApprovalStrategy
	path := strategyPath + "/" + url.PathEscape(strategyID)
	if err := c.doJSON(ctx, http.MethodPatch, path, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteJoinApprovalStrategy removes one strategy.
func (c *Client) DeleteJoinApprovalStrategy(ctx context.Context, strategyID string) error {
	if strategyID == "" {
		return errors.New("qqbotsdk: DeleteJoinApprovalStrategy needs a strategy id")
	}
	return c.doJSON(ctx, http.MethodDelete, strategyPath+"/"+url.PathEscape(strategyID), nil, nil, openAPICall)
}

// ExecuteJoinApprovalStrategy runs one strategy.
func (c *Client) ExecuteJoinApprovalStrategy(ctx context.Context, strategyID string) error {
	if strategyID == "" {
		return errors.New("qqbotsdk: ExecuteJoinApprovalStrategy needs a strategy id")
	}
	path := strategyPath + "/" + url.PathEscape(strategyID) + "/execute"
	return c.doJSON(ctx, http.MethodPost, path, nil, nil, openAPICall)
}

// UpdateJoinApprovalStrategyWhitelist adds up to 10000 numbers to a strategy's
// whitelist, or removes them.
func (c *Client) UpdateJoinApprovalStrategyWhitelist(ctx context.Context, strategyID string, req *UpdateWhitelistRequest) (*WhitelistResult, error) {
	if strategyID == "" {
		return nil, errors.New("qqbotsdk: UpdateJoinApprovalStrategyWhitelist needs a strategy id")
	}
	if req == nil {
		return nil, errors.New("qqbotsdk: UpdateJoinApprovalStrategyWhitelist needs a request")
	}
	switch req.Op {
	case ListOpAdd, ListOpDelete:
	case "":
		return nil, errors.New("qqbotsdk: op is required, " + ListOpAdd + " or " + ListOpDelete)
	default:
		return nil, fmt.Errorf("qqbotsdk: op %q is not %s or %s", req.Op, ListOpAdd, ListOpDelete)
	}
	if len(req.WhitelistUsers) == 0 {
		return nil, errors.New("qqbotsdk: UpdateJoinApprovalStrategyWhitelist needs at least one number")
	}
	if len(req.WhitelistUsers) > MaxWhitelistUsers {
		return nil, fmt.Errorf("qqbotsdk: UpdateJoinApprovalStrategyWhitelist takes at most %d numbers, got %d",
			MaxWhitelistUsers, len(req.WhitelistUsers))
	}

	var out WhitelistResult
	path := strategyPath + "/" + url.PathEscape(strategyID) + "/whitelist_users"
	if err := c.doJSON(ctx, http.MethodPost, path, req, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// validateOneGroupForm enforces the documented rule that a strategy and its
// group operations take the group openids or the QQ group numbers, never both
// and never neither.
func validateOneGroupForm(method string, openIDs, numbers, maxSize int) error {
	switch {
	case openIDs > 0 && numbers > 0:
		return fmt.Errorf("qqbotsdk: %s takes group_openids or group_ids, not both", method)
	case openIDs == 0 && numbers == 0:
		return fmt.Errorf("qqbotsdk: %s needs group_openids or group_ids", method)
	case openIDs > maxSize || numbers > maxSize:
		return fmt.Errorf("qqbotsdk: %s covers at most %d groups", method, maxSize)
	}
	return nil
}
