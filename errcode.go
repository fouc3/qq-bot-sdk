package qqbotsdk

// OpenAPIErrorCode is an err_code returned by the OpenAPI in a failure body.
//
// The value 0 means success. Codes are documented per endpoint; the ones
// defined here are the common codes listed in the API call guide.
type OpenAPIErrorCode int

// String returns the documented symbolic name of the code, or "" when the code
// is unknown. Only codes named in the API call guide have a name.
func (c OpenAPIErrorCode) String() string {
	if name, ok := openAPIErrorNames[c]; ok {
		return name
	}
	return ""
}

// openAPIErrorNames maps codes to the symbolic names used by the documentation.
var openAPIErrorNames = map[OpenAPIErrorCode]string{
	ErrUnknownAccount:             "UnknownAccount",
	ErrUnknownChannel:             "UnknownChannel",
	ErrUnknownGuild:               "UnknownGuild",
	ErrCheckAdminFailed:           "ErrorCheckAdminFailed",
	ErrCheckAdminNotPass:          "ErrorCheckAdminNotPass",
	ErrWrongAppID:                 "ErrorWrongAppid",
	ErrMissingAppID:               "ErrorWrongAppid",
	ErrNoAppID:                    "ErrorWrongAppid",
	ErrCheckAppPrivilegeFailed:    "ErrorCheckAppPrivilegeFailed",
	ErrCheckAppPrivilegeNotPass:   "ErrorCheckAppPrivilegeNotPass",
	ErrInterfaceForbidden:         "ErrorInterfaceForbidden",
	ErrCheckRobot:                 "ErrorCheckRobot",
	ErrCheckGuildAuth:             "ErrorCheckGuildAuth",
	ErrGuildAuthNotPass:           "ErrorGuildAuthNotPass",
	ErrRobotHasBanned:             "ErrorRobotHasBaned",
	ErrWrongToken:                 "ErrorWrongToken",
	ErrCheckTokenFailed:           "ErrorCheckTokenFailed",
	ErrCheckTokenNotPass:          "ErrorCheckTokenNotPass",
	ErrCheckUserAuth:              "ErrorCheckUserAuth",
	ErrUserAuthNotPass:            "ErrorUserAuthNotPass",
	ErrGetHTTPHeader:              "ErrorGetHTTPHeader",
	ErrGetHeaderUIN:               "ErrorGetHeaderUIN",
	ErrGetNick:                    "ErrorGetNick",
	ErrGetAvatar:                  "ErrorGetAvatar",
	ErrGetGuildID:                 "ErrorGetGuildID",
	ErrGetGuildInfo:               "ErrorGetGuildInfo",
	ErrReplaceIDFailed:            "ReplaceIDFailed",
	ErrRequestInvalid:             "RequestInvalid",
	ErrResponseInvalid:            "ResponseInvalid",
	ErrChannelHitWriteRateLimit:   "ChannelHitWriteRateLimit",
	ErrCannotSendEmptyMessage:     "CannotSendEmptyMessage",
	ErrInvalidFormBody:            "InvalidFormBody",
	ErrMarkdownCombination:        "带有 markdown 消息只支持 markdown 或者 keyboard 组合",
	ErrNotSameChannel:             "非同频道同子频道",
	ErrGetMessageFailed:           "获取消息失败",
	ErrMessageTemplateTypeInvalid: "消息模版类型错误",
	ErrMarkdownEmpty:              "markdown 有空值",
	ErrMarkdownListTooLong:        "markdown 列表长达最大值",
	ErrGuildIDConvertFailed:       "guild_id 转换失败",
	ErrReplySelfMessage:           "不能回复机器人自己产生的消息",
	ErrNotAtBotMessage:            "非 at 机器人消息",
	ErrNotBotMessage:              "非机器人产生的消息 或者 at 机器人消息",
	ErrMessageIDEmpty:             "message id 不能为空",
	ErrOnlyKeyboardEditable:       "只能修改含有 keyboard 元素的消息",
	ErrKeyboardEmpty:              "修改消息时，keyboard 元素不能为空",
	ErrOnlyOwnMessageEditable:     "只能修改机器人自己发送的消息",
	ErrModifyMessageFailed:        "修改消息错误",
	ErrMarkdownTemplateParam:      "markdown 模版参数错误",
	ErrInvalidMarkdownContent:     "无效的 markdown content",
	ErrMarkdownNotAllowed:         "不允许发送 markdown content",
	ErrMarkdownSyntaxConflict:     "markdown 参数只支持原生语法或者模版二选一",
	ErrURLCallRetractParamInvalid: "param invalid 撤回消息参数错误",
	ErrMsgIDError:                 "msgid error 消息 id 错误",
	ErrGetMessageRetry:            "fail to get message 获取消息错误(可重试)",
	ErrNoPermissionDeleteMessage:  "no permission to delete message 没有撤回此消息的权限",
	ErrRetractMessageFailed:       "retract message error 消息撤回失败(可重试)",
	ErrGetChannelFailed:           "fail to get channel 获取子频道失败(可重试)",
	ErrSafeMessageRateLimited:     "安全打击：消息被限频",
	ErrSafeMessageSensitive:       "安全打击：内容涉及敏感，请返回修改",
	ErrSafeNoExperience:           "安全打击：抱歉，暂未获得新功能体验资格",
	ErrSafeHit:                    "安全打击",
	ErrSafeGroupGone:              "安全打击：该群已失效或当前群已不存在",
	ErrInternalSystem:             "系统内部错误",
	ErrCallerNotGroupMember:       "调用方不是群成员",
	ErrGetChannelNameFailed:       "获取指定频道名称失败",
	ErrHomeChannelNotAdmin:        "主页频道非管理员不允许发消息",
	ErrAtAuthFailed:               "@次数鉴权失败",
	ErrTinyIDToUinFailed:          "TinyId 转换 Uin 失败",
	ErrNotPrivateChannelMember:    "非私有频道成员",
	ErrNotWhitelistAppChannel:     "非白名单应用子频道",
	ErrTriggerChannelRateLimit:    "触发频道内限频",
	ErrOtherError:                 "其他错误",
	ErrEditMessageSafeHit:         "安全打击",
	ErrPushMessageAsyncOK:         "PUSH_MSG_ASYNC_OK 推送消息异步调用成功，等待人工审核",
	ErrReplyMessageAsyncOK:        "REPLY_MSG_ASYNC_OK 回复消息异步调用成功，等待人工审核",
}

// Common OpenAPI error codes from the API call guide.
//
// The documentation reuses the symbolic name ErrorWrongAppid for three
// different codes; they are kept apart here as ErrWrongAppID, ErrMissingAppID
// and ErrNoAppID, and all three report that name from String.
const (
	// General account, channel and guild errors.
	ErrUnknownAccount OpenAPIErrorCode = 10001
	ErrUnknownChannel OpenAPIErrorCode = 10003
	ErrUnknownGuild   OpenAPIErrorCode = 10004

	// Administrator checks.
	ErrCheckAdminFailed  OpenAPIErrorCode = 11281
	ErrCheckAdminNotPass OpenAPIErrorCode = 11282

	// Application and AppID checks.
	ErrWrongAppID               OpenAPIErrorCode = 11251
	ErrCheckAppPrivilegeFailed  OpenAPIErrorCode = 11252
	ErrCheckAppPrivilegeNotPass OpenAPIErrorCode = 11253
	ErrInterfaceForbidden       OpenAPIErrorCode = 11254
	ErrMissingAppID             OpenAPIErrorCode = 11261
	ErrCheckRobot               OpenAPIErrorCode = 11262
	ErrCheckGuildAuth           OpenAPIErrorCode = 11263
	ErrGuildAuthNotPass         OpenAPIErrorCode = 11264
	ErrRobotHasBanned           OpenAPIErrorCode = 11265

	// Token checks.
	ErrWrongToken        OpenAPIErrorCode = 11241
	ErrCheckTokenFailed  OpenAPIErrorCode = 11242
	ErrCheckTokenNotPass OpenAPIErrorCode = 11243

	// User authorization checks.
	ErrCheckUserAuth   OpenAPIErrorCode = 11273
	ErrUserAuthNotPass OpenAPIErrorCode = 11274
	ErrNoAppID         OpenAPIErrorCode = 11275

	// HTTP header and profile lookups.
	ErrGetHTTPHeader OpenAPIErrorCode = 11301
	ErrGetHeaderUIN  OpenAPIErrorCode = 11302
	ErrGetNick       OpenAPIErrorCode = 11303
	ErrGetAvatar     OpenAPIErrorCode = 11304
	ErrGetGuildID    OpenAPIErrorCode = 11305
	ErrGetGuildInfo  OpenAPIErrorCode = 11306

	// ID replacement and payload validation.
	ErrReplaceIDFailed OpenAPIErrorCode = 12001
	ErrRequestInvalid  OpenAPIErrorCode = 12002
	ErrResponseInvalid OpenAPIErrorCode = 12003

	// Channel write rate limit.
	ErrChannelHitWriteRateLimit OpenAPIErrorCode = 20028

	// Message body and message editing errors.
	ErrCannotSendEmptyMessage     OpenAPIErrorCode = 50006
	ErrInvalidFormBody            OpenAPIErrorCode = 50035
	ErrMarkdownCombination        OpenAPIErrorCode = 50037
	ErrNotSameChannel             OpenAPIErrorCode = 50038
	ErrGetMessageFailed           OpenAPIErrorCode = 50039
	ErrMessageTemplateTypeInvalid OpenAPIErrorCode = 50040
	ErrMarkdownEmpty              OpenAPIErrorCode = 50041
	ErrMarkdownListTooLong        OpenAPIErrorCode = 50042
	ErrGuildIDConvertFailed       OpenAPIErrorCode = 50043
	ErrReplySelfMessage           OpenAPIErrorCode = 50045
	ErrNotAtBotMessage            OpenAPIErrorCode = 50046
	ErrNotBotMessage              OpenAPIErrorCode = 50047
	ErrMessageIDEmpty             OpenAPIErrorCode = 50048
	ErrOnlyKeyboardEditable       OpenAPIErrorCode = 50049
	ErrKeyboardEmpty              OpenAPIErrorCode = 50050
	ErrOnlyOwnMessageEditable     OpenAPIErrorCode = 50051
	ErrModifyMessageFailed        OpenAPIErrorCode = 50053
	ErrMarkdownTemplateParam      OpenAPIErrorCode = 50054
	ErrInvalidMarkdownContent     OpenAPIErrorCode = 50055
	ErrMarkdownNotAllowed         OpenAPIErrorCode = 50056
	ErrMarkdownSyntaxConflict     OpenAPIErrorCode = 50057

	// Message retraction.
	ErrURLCallRetractParamInvalid OpenAPIErrorCode = 306001
	ErrMsgIDError                 OpenAPIErrorCode = 306002
	ErrGetMessageRetry            OpenAPIErrorCode = 306003
	ErrNoPermissionDeleteMessage  OpenAPIErrorCode = 306004
	ErrRetractMessageFailed       OpenAPIErrorCode = 306005
	ErrGetChannelFailed           OpenAPIErrorCode = 306006

	// Safe-hitting (content moderation) errors.
	ErrSafeMessageRateLimited  OpenAPIErrorCode = 1100100
	ErrSafeMessageSensitive    OpenAPIErrorCode = 1100101
	ErrSafeNoExperience        OpenAPIErrorCode = 1100102
	ErrSafeHit                 OpenAPIErrorCode = 1100103
	ErrSafeGroupGone           OpenAPIErrorCode = 1100104
	ErrInternalSystem          OpenAPIErrorCode = 1100300
	ErrCallerNotGroupMember    OpenAPIErrorCode = 1100301
	ErrGetChannelNameFailed    OpenAPIErrorCode = 1100302
	ErrHomeChannelNotAdmin     OpenAPIErrorCode = 1100303
	ErrAtAuthFailed            OpenAPIErrorCode = 1100304
	ErrTinyIDToUinFailed       OpenAPIErrorCode = 1100305
	ErrNotPrivateChannelMember OpenAPIErrorCode = 1100306
	ErrNotWhitelistAppChannel  OpenAPIErrorCode = 1100307
	ErrTriggerChannelRateLimit OpenAPIErrorCode = 1100308
	ErrOtherError              OpenAPIErrorCode = 1100499
	ErrEditMessageSafeHit      OpenAPIErrorCode = 3300006

	// Asynchronous message delivery accepted for review.
	ErrPushMessageAsyncOK  OpenAPIErrorCode = 304023
	ErrReplyMessageAsyncOK OpenAPIErrorCode = 304024
)
