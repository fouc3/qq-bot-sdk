package qqbotsdk

// Error codes documented by the message endpoints. They are kept apart from
// the common codes in errcode.go so each endpoint's own table stays readable.
const (
	// Message type and content validation.
	ErrMsgTypeMismatch       OpenAPIErrorCode = 22006
	ErrInputTypeInvalid      OpenAPIErrorCode = 50059
	ErrMessageContentInvalid OpenAPIErrorCode = 304061
	ErrMsgTypeInvalid        OpenAPIErrorCode = 340069
	ErrFileInfoInvalid       OpenAPIErrorCode = 304080

	// Keyboard and subscription buttons.
	ErrSubscribeButtonLimit         OpenAPIErrorCode = 304062
	ErrSubscribeMessageUnauthorized OpenAPIErrorCode = 304064
	ErrKeyboardStyleInvalid         OpenAPIErrorCode = 305007

	// Message identity and reply windows.
	ErrMessageIDExpiredCannotReply OpenAPIErrorCode = 304103
	ErrReplyMsgIDExpired           OpenAPIErrorCode = 40034005
	ErrReplyMsgIDInvalid           OpenAPIErrorCode = 40034024
	ErrEventIDInvalid              OpenAPIErrorCode = 40034025
	ErrEventIDExpired              OpenAPIErrorCode = 40034026
	ErrEventNotRepliable           OpenAPIErrorCode = 40034027
	ErrPassiveReplyLimitExceeded   OpenAPIErrorCode = 40034128
	ErrGetBotInfoFailed            OpenAPIErrorCode = 340067

	// Content moderation and richness.
	ErrMessageContentViolation OpenAPIErrorCode = 40034006
	ErrRichMediaTransferFailed OpenAPIErrorCode = 40034004

	// Markdown content.
	ErrMarkdownParamEmptyValue      OpenAPIErrorCode = 40034008
	ErrMarkdownParamNewline         OpenAPIErrorCode = 40034009
	ErrTemplateParamHasMarkdown     OpenAPIErrorCode = 40034010
	ErrInvalidMarkdownBody          OpenAPIErrorCode = 40034011
	ErrMarkdownParamError           OpenAPIErrorCode = 40034124
	ErrMarkdownTemplateNoPermission OpenAPIErrorCode = 40034127

	// Keyboard layout limits and command buttons.
	ErrKeyboardSizeExceeded    OpenAPIErrorCode = 40034029
	ErrCommandTypeUnsupported  OpenAPIErrorCode = 40034106
	ErrCommandParamTooLong     OpenAPIErrorCode = 40034108
	ErrCommandParamParseFailed OpenAPIErrorCode = 40034109

	// Active messages and wake-up recalls.
	ErrActiveMessageRateLimited  OpenAPIErrorCode = 40034100
	ErrActiveMessageNoPermission OpenAPIErrorCode = 40034105
	ErrWakeupLimitReached        OpenAPIErrorCode = 40034122
	ErrWakeupUnsupported         OpenAPIErrorCode = 40034123

	// Group and friend relationship.
	ErrBotNotGroupMember          OpenAPIErrorCode = 40034101
	ErrBotMuted                   OpenAPIErrorCode = 40054002
	ErrBotNotInGroup              OpenAPIErrorCode = 40054003
	ErrNoFriendRelation           OpenAPIErrorCode = 40054004
	ErrVerifyFriendRelationFailed OpenAPIErrorCode = 40054006
	ErrURLNotAllowedInMessage     OpenAPIErrorCode = 40054010
	ErrUserRejectedMessage        OpenAPIErrorCode = 40054013
	ErrBotOffline                 OpenAPIErrorCode = 40054016

	// Deduplication and length.
	ErrMessageDeduplicated      OpenAPIErrorCode = 40054005
	ErrMessageTooLong           OpenAPIErrorCode = 40054007
	ErrMessageTooLongOrAbnormal OpenAPIErrorCode = 40054018

	// Transport failures reported as business codes.
	ErrMessageSendFailedGroup OpenAPIErrorCode = 50055001
	ErrMessageSendFailedC2C   OpenAPIErrorCode = 50055002
	ErrARKMessageSendFailed   OpenAPIErrorCode = 50055006

	// Streaming messages.
	ErrStreamPrefixImmutable OpenAPIErrorCode = 40007
	ErrStreamInternalError   OpenAPIErrorCode = 50001
	ErrStreamRateLimited     OpenAPIErrorCode = 50002

	// Rich media upload.
	ErrUnsupportedOperation      OpenAPIErrorCode = 10000
	ErrFileGroupOrBotMuted       OpenAPIErrorCode = 850018
	ErrFileFormatUnsupported     OpenAPIErrorCode = 850019
	ErrFileDownloadFailed        OpenAPIErrorCode = 850026
	ErrFileSendTimeout           OpenAPIErrorCode = 850027
	ErrFileTooLarge              OpenAPIErrorCode = 850031
	ErrFileUploadFailed          OpenAPIErrorCode = 40093001
	ErrFileDailyCapacityExceeded OpenAPIErrorCode = 40093002

	// Recall.
	ErrUserOpenIDInvalid  OpenAPIErrorCode = 306009
	ErrRecallParamInvalid OpenAPIErrorCode = 40061001
	ErrRecallMsgIDInvalid OpenAPIErrorCode = 40061002
	ErrRecallNoPermission OpenAPIErrorCode = 40062003
	ErrRecallTimeExceeded OpenAPIErrorCode = 40064004
	ErrRecallFailed       OpenAPIErrorCode = 50065001
)

// messageErrorNames maps the message endpoint codes to their documented text.
//
// OpenAPIErrorCode.String consults it after the common table.
var messageErrorNames = map[OpenAPIErrorCode]string{
	ErrMsgTypeMismatch:              "消息类型与内容不匹配",
	ErrInputTypeInvalid:             "输入类型错误",
	ErrMessageContentInvalid:        "消息内容无效",
	ErrMsgTypeInvalid:               "消息类型无效",
	ErrFileInfoInvalid:              "文件信息无效",
	ErrSubscribeButtonLimit:         "订阅按钮数量达到上限",
	ErrSubscribeMessageUnauthorized: "订阅消息未授权",
	ErrKeyboardStyleInvalid:         "键盘样式参数错误",
	ErrMessageIDExpiredCannotReply:  "消息ID已过期，不能回复",
	ErrReplyMsgIDExpired:            "回复消息msg_id已过期",
	ErrReplyMsgIDInvalid:            "请求参数msg_id无效或越权",
	ErrEventIDInvalid:               "请求参数event_id无效",
	ErrEventIDExpired:               "请求参数event_id已过期",
	ErrEventNotRepliable:            "该事件不支持回复消息",
	ErrPassiveReplyLimitExceeded:    "被动回复时间或次数超限",
	ErrGetBotInfoFailed:             "获取机器人信息失败",
	ErrMessageContentViolation:      "消息内容违规",
	ErrRichMediaTransferFailed:      "富媒体信息转存失败",
	ErrMarkdownParamEmptyValue:      "markdown参数有空值",
	ErrMarkdownParamNewline:         "markdown参数有换行符",
	ErrTemplateParamHasMarkdown:     "模版参数中不能含有markdown语法",
	ErrInvalidMarkdownBody:          "无效的markdown内容",
	ErrMarkdownParamError:           "markdown消息参数错误",
	ErrMarkdownTemplateNoPermission: "无markdown模板权限",
	ErrKeyboardSizeExceeded:         "内联键盘行/列超限",
	ErrCommandTypeUnsupported:       "消息不支持该指令类型",
	ErrCommandParamTooLong:          "指令参数长度超限",
	ErrCommandParamParseFailed:      "指令参数解析失败",
	ErrActiveMessageRateLimited:     "主动消息发送超过频控限制",
	ErrActiveMessageNoPermission:    "主动消息发送失败，无权限",
	ErrWakeupLimitReached:           "召回消息已达区间上限",
	ErrWakeupUnsupported:            "不支持召回消息",
	ErrBotNotGroupMember:            "机器人非群成员",
	ErrBotMuted:                     "机器人被禁言",
	ErrBotNotInGroup:                "机器人不是群成员",
	ErrNoFriendRelation:             "无好友关系",
	ErrVerifyFriendRelationFailed:   "验证好友关系失败",
	ErrURLNotAllowedInMessage:       "不允许发送URL",
	ErrUserRejectedMessage:          "用户拒收消息",
	ErrBotOffline:                   "机器人已下线",
	ErrMessageDeduplicated:          "消息被去重",
	ErrMessageTooLong:               "消息长度超限",
	ErrMessageTooLongOrAbnormal:     "消息过长或异常",
	ErrMessageSendFailedGroup:       "消息发送异常，请稍后重试",
	ErrMessageSendFailedC2C:         "消息发送异常，请稍后重试",
	ErrARKMessageSendFailed:         "ARK消息发送异常，请稍后重试",
	ErrStreamPrefixImmutable:        "已下发内容前缀不可修改",
	ErrStreamInternalError:          "服务内部错误",
	ErrStreamRateLimited:            "频率限制",
	ErrUnsupportedOperation:         "不支持的操作",
	ErrFileGroupOrBotMuted:          "群被禁言或者机器人被禁言",
	ErrFileFormatUnsupported:        "不支持的文件格式",
	ErrFileDownloadFailed:           "下载原始文件失败",
	ErrFileSendTimeout:              "发送数据超时",
	ErrFileTooLarge:                 "上传文件超过大小限制",
	ErrFileUploadFailed:             "文件上传失败，请重试",
	ErrFileDailyCapacityExceeded:    "超过今天发送文件容量上限",
	ErrUserOpenIDInvalid:            "用户openid无效",
	ErrRecallParamInvalid:           "请求参数无效",
	ErrRecallMsgIDInvalid:           "请求参数msgid无效",
	ErrRecallNoPermission:           "无操作权限",
	ErrRecallTimeExceeded:           "已超出消息撤回时限",
	ErrRecallFailed:                 "消息撤回失败，请稍后重试",
}
