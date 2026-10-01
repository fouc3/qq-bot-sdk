package qqbotsdk

// Channel types, as documented for ChannelInfo.Type.
const (
	// ChannelTypeText is a text channel.
	ChannelTypeText = 0
	// ChannelTypeVoice is a voice channel.
	ChannelTypeVoice = 2
	// ChannelTypeGroup is a channel category.
	ChannelTypeGroup = 4
	// ChannelTypeLive is a live streaming channel.
	ChannelTypeLive = 10005
	// ChannelTypeApplication is an application channel.
	ChannelTypeApplication = 10006
	// ChannelTypeForum is a forum channel.
	ChannelTypeForum = 10007
)

// ChannelInfo describes one channel, as carried by the channel create, update
// and delete events.
type ChannelInfo struct {
	// ID is the channel id.
	ID string `json:"id"`
	// GuildID is the guild the channel belongs to.
	GuildID string `json:"guild_id"`
	// Name is the channel name.
	Name string `json:"name"`
	// Type is one of the ChannelType constants.
	Type int `json:"type,omitempty"`
	// SubType is the channel sub type.
	SubType int `json:"sub_type,omitempty"`
	// OwnerID is the id of the channel creator.
	OwnerID string `json:"owner_id,omitempty"`
	// OpUserID is the id of the operator.
	OpUserID string `json:"op_user_id,omitempty"`
}
