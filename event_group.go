package qqbotsdk

// Friend add scene values, as documented for FriendAddData.Scene.
const (
	// FriendSceneDefault is the default scene.
	FriendSceneDefault = 1000
	// FriendSceneSearchAll is a search hit on the all tab.
	FriendSceneSearchAll = 1001
	// FriendSceneSearchBot is a search hit on the bot tab.
	FriendSceneSearchBot = 1002
	// FriendSceneGroup is addition from a group.
	FriendSceneGroup = 1003
	// FriendSceneZone is addition from Qzone.
	FriendSceneZone = 1004
	// FriendSceneProfileInApp is the in-app profile page.
	FriendSceneProfileInApp = 2001
	// FriendSceneProfileOutApp is the out-of-app profile page.
	FriendSceneProfileOutApp = 2002
	// FriendSceneShareLinkInApp is a developer share link opened in app.
	FriendSceneShareLinkInApp = 2003
	// FriendSceneShareLinkOutApp is a developer share link opened out of app.
	FriendSceneShareLinkOutApp = 2004
)

// Group member roles, as documented for User.MemberRole.
const (
	// GroupMemberRoleMember is an ordinary member.
	GroupMemberRoleMember = "member"
	// GroupMemberRoleAdmin is an administrator.
	GroupMemberRoleAdmin = "admin"
	// GroupMemberRoleOwner is the group owner.
	GroupMemberRoleOwner = "owner"
)

// Group join request sources, as documented for GroupJoinRequestData.
const (
	// GroupJoinSourceSelf applied on the user's own initiative.
	GroupJoinSourceSelf = "self_apply"
	// GroupJoinSourceInvited came from an invitation.
	GroupJoinSourceInvited = "invited"
)

// Group join verification methods.
const (
	// GroupVerifyMessage verifies with a message.
	GroupVerifyMessage = "verify_message"
	// GroupVerifyAdminReviewQA verifies with questions set by an administrator.
	GroupVerifyAdminReviewQA = "admin_review_qa"
)

// Subscription authorization operations, as documented for
// SubscribeMsgTemplateResult.Op.
const (
	// SubscribeOpAllowed means the user allowed the template.
	SubscribeOpAllowed = 1
	// SubscribeOpRejected means the user rejected the template.
	SubscribeOpRejected = 2
)

// C2CMsgReceiveData is the body of C2C_MSG_RECEIVE: a user turned on proactive
// message push for this bot.
type C2CMsgReceiveData struct {
	// Timestamp is the Unix second of the operation.
	Timestamp int64 `json:"timestamp"`
	// OpenID identifies the user.
	OpenID string `json:"openid"`
}

// C2CMsgRejectData is the body of C2C_MSG_REJECT: a user turned proactive
// message push off, so the bot can no longer start a conversation with them.
type C2CMsgRejectData struct {
	// Timestamp is the Unix second of the operation.
	Timestamp int64 `json:"timestamp"`
	// OpenID identifies the user.
	OpenID string `json:"openid"`
}

// FriendAuthor is the cross-application identity in a friend event.
type FriendAuthor struct {
	// UnionOpenID is the cross-application user OpenID.
	UnionOpenID string `json:"union_openid,omitempty"`
}

// FriendAddData is the body of FRIEND_ADD: a user added the bot as a friend.
type FriendAddData struct {
	// Timestamp is the Unix second of the addition.
	Timestamp int64 `json:"timestamp"`
	// OpenID identifies the user.
	OpenID string `json:"openid"`
	// Scene is one of the FriendScene constants.
	Scene int `json:"scene,omitempty"`
	// SceneParam is the callback data carried by the share link that led here.
	SceneParam string `json:"scene_param,omitempty"`
	// Author is the cross-application identity of the user.
	Author *FriendAuthor `json:"author,omitempty"`
	// ShortCode is the short code of the bot share link.
	ShortCode string `json:"short_code,omitempty"`
}

// FriendDelData is the body of FRIEND_DEL: a user removed the bot.
type FriendDelData struct {
	// Timestamp is the Unix second of the deletion.
	Timestamp int64 `json:"timestamp"`
	// OpenID identifies the user.
	OpenID string `json:"openid"`
	// Author is the cross-application identity of the user.
	Author *FriendAuthor `json:"author,omitempty"`
}

// GroupAddRobotData is the body of GROUP_ADD_ROBOT: the bot was added to a
// group.
type GroupAddRobotData struct {
	// Timestamp is the Unix second of the addition.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// OpMemberOpenID is the member who added the bot.
	OpMemberOpenID string `json:"op_member_openid,omitempty"`
}

// GroupDelRobotData is the body of GROUP_DEL_ROBOT: the bot was removed from a
// group.
type GroupDelRobotData struct {
	// Timestamp is the Unix second of the removal.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// OpMemberOpenID is the member who removed the bot.
	OpMemberOpenID string `json:"op_member_openid,omitempty"`
}

// GroupMemberAddData is the body of GROUP_MEMBER_ADD: a member joined.
type GroupMemberAddData struct {
	// Timestamp is the Unix second of the event.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// MemberOpenID is the group member OpenID of the new member.
	MemberOpenID string `json:"member_openid,omitempty"`
	// UserOpenID is the cross-application user OpenID, and may be empty.
	UserOpenID string `json:"user_openid,omitempty"`
}

// GroupMemberRemoveData is the body of GROUP_MEMBER_REMOVE: a member left or
// was removed.
type GroupMemberRemoveData struct {
	// Timestamp is the Unix second of the event.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// MemberOpenID is the group member OpenID of the member who left.
	MemberOpenID string `json:"member_openid,omitempty"`
	// UserOpenID is the cross-application user OpenID, and may be empty.
	UserOpenID string `json:"user_openid,omitempty"`
}

// GroupMsgReceiveData is the body of GROUP_MSG_RECEIVE: a group administrator
// turned notifications on.
type GroupMsgReceiveData struct {
	// Timestamp is the Unix second of the operation.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// OpMemberOpenID is the administrator who made the change.
	OpMemberOpenID string `json:"op_member_openid,omitempty"`
}

// GroupMsgRejectData is the body of GROUP_MSG_REJECT: a group administrator
// turned notifications off.
type GroupMsgRejectData struct {
	// Timestamp is the Unix second of the operation.
	Timestamp int64 `json:"timestamp"`
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// OpMemberOpenID is the administrator who made the change.
	OpMemberOpenID string `json:"op_member_openid,omitempty"`
}

// ReviewQA is one verification question and the answer the applicant gave.
type ReviewQA struct {
	// Question is set by the administrator.
	Question string `json:"question,omitempty"`
	// Answer is filled in by the applicant.
	Answer string `json:"answer,omitempty"`
}

// VerifyInfo describes how a group join request is verified.
type VerifyInfo struct {
	// Method is GroupVerifyMessage or GroupVerifyAdminReviewQA.
	Method string `json:"method,omitempty"`
	// VerifyMessage is the verification message, and may be present when
	// Method is GroupVerifyMessage.
	VerifyMessage string `json:"verify_message,omitempty"`
	// ReviewQAList is present when Method is GroupVerifyAdminReviewQA.
	ReviewQAList []ReviewQA `json:"review_qa_list,omitempty"`
}

// AutoApproved carries the strategy of an automatically approved join request.
//
// The documentation names this structure "AutoAppproved"; the misspelling is
// not reproduced here.
type AutoApproved struct {
	// StrategyID is the id of the strategy that approved the request.
	StrategyID string `json:"strategy_id,omitempty"`
}

// GroupJoinRequestData is the body of GROUP_JOIN_REQUEST: a user asked to join
// a group.
//
// This event is only delivered while the bot is a group administrator.
type GroupJoinRequestData struct {
	// GroupOpenID identifies the group.
	GroupOpenID string `json:"group_openid"`
	// JoinRequestID must be returned when answering the request.
	JoinRequestID string `json:"join_request_id"`
	// RiskTips is a safety hint: warning_tips for a suspicious message, or
	// top_tips when the message matched sec_risk_rules.
	RiskTips string `json:"risk_tips,omitempty"`
	// UnionOpenID is the cross-application identity of the applicant.
	UnionOpenID string `json:"union_openid,omitempty"`
	// MemberOpenID identifies the applicant.
	MemberOpenID string `json:"member_openid,omitempty"`
	// Username is the applicant's nickname.
	Username string `json:"username,omitempty"`
	// ApplyAt is the request time in RFC3339.
	ApplyAt string `json:"apply_at,omitempty"`
	// ApplySource is GroupJoinSourceSelf or GroupJoinSourceInvited.
	ApplySource string `json:"apply_source,omitempty"`
	// InvitedBy is the inviter, present when ApplySource is
	// GroupJoinSourceInvited.
	InvitedBy string `json:"invited_by,omitempty"`
	// Bot reports whether the applicant is a bot account.
	Bot bool `json:"bot,omitempty"`
	// VerifyInfo describes the verification method.
	VerifyInfo *VerifyInfo `json:"verify_info,omitempty"`
	// AutoApproved is only present on a downlink event that was approved
	// automatically.
	AutoApproved *AutoApproved `json:"auto_approved,omitempty"`
}

// SubscribeMsgTemplateResult is one template's authorization result.
type SubscribeMsgTemplateResult struct {
	// TemplateID is the platform template id.
	TemplateID int64 `json:"template_id,omitempty"`
	// CustomTemplateID is the developer defined template id.
	CustomTemplateID string `json:"custom_template_id,omitempty"`
	// Op is SubscribeOpAllowed or SubscribeOpRejected.
	Op int `json:"op,omitempty"`
	// SubscribeID is needed when sending a subscription message.
	SubscribeID string `json:"subscribe_id,omitempty"`
	// SubscribeTS is the Unix second of the subscription.
	SubscribeTS int64 `json:"subscribe_ts,omitempty"`
	// UpdateTS is the Unix second of the last status change.
	UpdateTS int64 `json:"update_ts,omitempty"`
}

// SubscribeMessageStatusData is the body of SUBSCRIBE_MESSAGE_STATUS: a user's
// authorization for subscription templates changed.
type SubscribeMessageStatusData struct {
	// GroupOpenID is set for a group subscription.
	GroupOpenID string `json:"group_openid,omitempty"`
	// OpenID is set for a personal subscription.
	OpenID string `json:"openid,omitempty"`
	// Result holds one entry per template.
	Result []SubscribeMsgTemplateResult `json:"result,omitempty"`
}
