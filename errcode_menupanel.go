package qqbotsdk

// Error codes documented by the custom menu and instruction panel endpoints.
const (
	// ErrMenuPanelParamInvalid means a request parameter was wrong.
	ErrMenuPanelParamInvalid OpenAPIErrorCode = 40030001
	// ErrPanelNotFound means no panel exists for the given panel_id.
	ErrPanelNotFound OpenAPIErrorCode = 40030006
	// ErrURLFormatInvalid means a link does not start with https://.
	ErrURLFormatInvalid OpenAPIErrorCode = 40030008
	// ErrPanelOperationInProgress means another change to the panel is still
	// running, and the call may be retried.
	ErrPanelOperationInProgress OpenAPIErrorCode = 40030009
	// ErrPanelScopeInvalid means scope is not one of c2c, group, channel or dm.
	ErrPanelScopeInvalid OpenAPIErrorCode = 40030011
	// ErrPanelTargetTypeInvalid means target_type is not all or specific, or
	// is specific for a scope that only supports all.
	ErrPanelTargetTypeInvalid OpenAPIErrorCode = 40030012
	// ErrCountLimitExceeded means too many items or targets were sent. The
	// response message carries the limit.
	ErrCountLimitExceeded OpenAPIErrorCode = 40030013
	// ErrMenuTypeInvalid means a menu type is not one of switch, send_message,
	// link or menu.
	ErrMenuTypeInvalid OpenAPIErrorCode = 40030014
	// ErrPanelItemTypeInvalid means a panel item type is not command or link.
	ErrPanelItemTypeInvalid OpenAPIErrorCode = 40030015
	// ErrRequiredFieldMissing means a required field was not sent.
	ErrRequiredFieldMissing OpenAPIErrorCode = 40030016
	// ErrPanelTargetOpInvalid means op is not add or del.
	ErrPanelTargetOpInvalid OpenAPIErrorCode = 40030017
	// ErrScopeNotSupported means the scope does not support the operation.
	ErrScopeNotSupported OpenAPIErrorCode = 40030018
	// ErrContentSecurityRisk means the platform rejected the content as
	// sensitive.
	ErrContentSecurityRisk OpenAPIErrorCode = 40030020
	// ErrGlobalPanelNoTarget means a global panel cannot gain associated
	// targets; a specific panel is required instead.
	ErrGlobalPanelNoTarget OpenAPIErrorCode = 40030021
)

// menuPanelErrorNames maps the menu and panel codes to their documented text.
var menuPanelErrorNames = map[OpenAPIErrorCode]string{
	ErrMenuPanelParamInvalid:    "参数错误",
	ErrPanelNotFound:            "指令面板不存在",
	ErrURLFormatInvalid:         "URL 格式错误",
	ErrPanelOperationInProgress: "指令面板操作进行中，请稍后重试",
	ErrPanelScopeInvalid:        "生效场景不合法",
	ErrPanelTargetTypeInvalid:   "生效范围不合法",
	ErrCountLimitExceeded:       "超出数量限制",
	ErrMenuTypeInvalid:          "菜单类型不合法",
	ErrPanelItemTypeInvalid:     "面板元素类型不合法",
	ErrRequiredFieldMissing:     "必填字段缺失",
	ErrPanelTargetOpInvalid:     "操作类型不合法",
	ErrScopeNotSupported:        "当前场景不支持此操作",
	ErrContentSecurityRisk:      "内容存在安全风险，请修改后重试",
	ErrGlobalPanelNoTarget:      "全局面板不支持添加指定关联对象",
}
