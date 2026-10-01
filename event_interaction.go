package qqbotsdk

// Interaction types carried by INTERACTION_CREATE.
const (
	// InteractionInlineKeyboard is a message inline keyboard button click.
	InteractionInlineKeyboard = 11
	// InteractionCallbackCommand is a single chat custom menu click.
	InteractionCallbackCommand = 12
	// InteractionMessageFeedback is a like or dislike on an agent message.
	InteractionMessageFeedback = 13
	// InteractionClearSession is a session history clearing.
	InteractionClearSession = 14
	// InteractionInOutStory is entering or leaving a story.
	InteractionInOutStory = 15
	// InteractionSwitchModel is a model switch.
	InteractionSwitchModel = 16
	// InteractionUserAuthorize is a user authorization.
	InteractionUserAuthorize = 18
	// InteractionGroupAuthorize is a group authorization.
	InteractionGroupAuthorize = 19
	// InteractionGroupAuthorizeStatus is a group authorization status change.
	InteractionGroupAuthorizeStatus = 20
)

// Chat types carried by InteractionCreateData.ChatType.
const (
	// InteractionChatGuild is a channel.
	InteractionChatGuild = 0
	// InteractionChatGroup is a group.
	InteractionChatGroup = 1
	// InteractionChatC2C is a single chat.
	InteractionChatC2C = 2
)

// Scenes carried by InteractionCreateData.Scene.
const (
	// InteractionSceneC2C is a single chat.
	InteractionSceneC2C = "c2c"
	// InteractionSceneGroup is a group.
	InteractionSceneGroup = "group"
	// InteractionSceneGuild is a channel.
	InteractionSceneGuild = "guild"
)

// Message feedback options, as documented for InteractionResolved.FeedbackOpt.
const (
	// InteractionFeedbackLike is a like.
	InteractionFeedbackLike = "LIKE"
	// InteractionFeedbackUnlike is a dislike.
	InteractionFeedbackUnlike = "UNLIKE"
)

// Story actions, as documented for InteractionResolved.Action.
const (
	// InteractionEnterStory enters a story.
	InteractionEnterStory = "ENTER_STORY"
	// InteractionQuitStory leaves a story.
	InteractionQuitStory = "QUIT_STORY"
)

// Authorization scenes and scopes.
const (
	// InteractionAuthSceneSetting is authorization from a profile page.
	InteractionAuthSceneSetting = "setting"
	// InteractionAuthSceneDialog is authorization from a dialog.
	InteractionAuthSceneDialog = "dialog"
	// InteractionAuthScopeC2CPush is single chat proactive message push.
	InteractionAuthScopeC2CPush = "c2c_push"
	// InteractionAuthScopeGroupPush is group proactive message push.
	InteractionAuthScopeGroupPush = "group_push"
)

// InteractionMessageScene is the message scene of a feedback interaction.
type InteractionMessageScene struct {
	// Ext holds key=value pairs, for example "disable_net_search=1".
	Ext []string `json:"ext,omitempty"`
}

// AuthorizeData is the body of a user or group authorization.
type AuthorizeData struct {
	// OptScene is InteractionAuthSceneSetting or InteractionAuthSceneDialog.
	OptScene string `json:"opt_scene,omitempty"`
	// Scope is InteractionAuthScopeC2CPush or InteractionAuthScopeGroupPush.
	Scope string `json:"scope,omitempty"`
}

// InteractionResolved holds the parsed interaction payload.
//
// Which field is populated depends on the outer interaction type.
type InteractionResolved struct {
	// ButtonData is the data field of a button, or the callback data of a
	// message feedback.
	ButtonData string `json:"button_data,omitempty"`
	// ButtonID is the id field of a button.
	ButtonID string `json:"button_id,omitempty"`
	// UserID is the acting user id, in a channel scenario.
	UserID string `json:"user_id,omitempty"`
	// FeatureID is the feature id of a custom menu, set in the console.
	FeatureID string `json:"feature_id,omitempty"`
	// MessageID is the acted-on message: a message OpenID in a channel, or
	// the bot message id in a feedback.
	MessageID string `json:"message_id,omitempty"`
	// FeedbackOpt is InteractionFeedbackLike or
	// InteractionFeedbackUnlike.
	FeedbackOpt string `json:"feedback_opt,omitempty"`
	// Checked reports whether the feedback option is selected.
	Checked int `json:"checked,omitempty"`
	// Action is InteractionEnterStory or InteractionQuitStory for a story, or
	// the action of a model switch.
	Action string `json:"action,omitempty"`
	// MessageScene is set for a message feedback.
	MessageScene *InteractionMessageScene `json:"message_scene,omitempty"`
	// AuthorizeData is set for a user or group authorization.
	AuthorizeData *AuthorizeData `json:"authorize_data,omitempty"`
}

// InteractionData is the interaction payload wrapper.
type InteractionData struct {
	// Type repeats the outer interaction type.
	Type int `json:"type,omitempty"`
	// Resolved holds the parsed interaction payload.
	Resolved *InteractionResolved `json:"resolved,omitempty"`
}

// InteractionCreateData is the body of INTERACTION_CREATE: a user interacted
// with the bot.
//
// An interaction of type InteractionInlineKeyboard or
// InteractionCallbackCommand must be answered by calling
// PUT /interactions/{interaction_id}, or the client keeps showing a loading
// state until it times out. Use NeedsResponse to check. The other types need no
// answer.
//
// An interaction_id may be answered only once, and expires after a timeout.
type InteractionCreateData struct {
	// ID is the event id, used to answer the interaction.
	ID string `json:"id"`
	// Type is one of the Interaction constants.
	Type int `json:"type"`
	// Scene is InteractionSceneC2C, InteractionSceneGroup or
	// InteractionSceneGuild.
	Scene string `json:"scene,omitempty"`
	// ChatType is one of the InteractionChat constants.
	ChatType int `json:"chat_type,omitempty"`
	// Timestamp is the trigger time in RFC3339.
	Timestamp string `json:"timestamp,omitempty"`
	// GuildID is set in a channel scenario.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is set in a channel scenario.
	ChannelID string `json:"channel_id,omitempty"`
	// UserOpenID is set in a single chat scenario.
	UserOpenID string `json:"user_openid,omitempty"`
	// GroupOpenID is set in a group scenario.
	GroupOpenID string `json:"group_openid,omitempty"`
	// GroupMemberOpenID is set in a group scenario.
	GroupMemberOpenID string `json:"group_member_openid,omitempty"`
	// Data is the interaction payload.
	Data *InteractionData `json:"data,omitempty"`
	// Version is the payload version, 1 by default.
	Version int `json:"version,omitempty"`
	// ApplicationID is the bot AppID.
	ApplicationID string `json:"application_id,omitempty"`
}

// NeedsResponse reports whether the interaction must be answered, which the
// documentation requires for a message button and a custom menu click.
func (i *InteractionCreateData) NeedsResponse() bool {
	if i == nil {
		return false
	}
	return i.Type == InteractionInlineKeyboard || i.Type == InteractionCallbackCommand
}
