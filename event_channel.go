package qqbotsdk

// ReactionTargetType values, as documented for ReactionTarget.Type.
const (
	// ReactionTargetMessage is a message.
	ReactionTargetMessage = 0
	// ReactionTargetPost is a forum post.
	ReactionTargetPost = 1
	// ReactionTargetComment is a comment.
	ReactionTargetComment = 2
	// ReactionTargetReply is a reply.
	ReactionTargetReply = 3
)

// Member is a user's membership of a guild, carried by a channel message and by
// the member events.
type Member struct {
	// User is the guild level user information. The member endpoints fill it
	// in, other payloads may omit it.
	User *User `json:"user,omitempty"`
	// Nick is the guild nickname.
	Nick string `json:"nick,omitempty"`
	// Roles are the id group ids the member holds, in this guild.
	Roles []string `json:"roles,omitempty"`
	// JoinedAt is when the user joined the guild, in ISO8601.
	JoinedAt string `json:"joined_at,omitempty"`
}

// GuildMessage is the channel message object carried by AT_MESSAGE_CREATE,
// MESSAGE_CREATE and DIRECT_MESSAGE_CREATE.
//
// It is deliberately not named Message: Message is the request body used when
// sending, and the two structures differ.
type GuildMessage struct {
	// ID is the message id.
	ID string `json:"id"`
	// ChannelID is the channel it was sent in.
	ChannelID string `json:"channel_id,omitempty"`
	// GuildID is the guild it was sent in.
	GuildID string `json:"guild_id,omitempty"`
	// Content is the message text.
	Content string `json:"content,omitempty"`
	// Timestamp is when it was created, in ISO8601.
	Timestamp string `json:"timestamp,omitempty"`
	// EditedTimestamp is when it was last edited, in ISO8601.
	EditedTimestamp string `json:"edited_timestamp,omitempty"`
	// MentionEveryone reports whether it mentions everyone.
	MentionEveryone bool `json:"mention_everyone,omitempty"`
	// Author is the sender.
	Author *User `json:"author,omitempty"`
	// Attachments are the attached files.
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	// Embeds are the embed cards.
	Embeds []MessageEmbed `json:"embeds,omitempty"`
	// Mentions are the mentioned users.
	Mentions []User `json:"mentions,omitempty"`
	// Member is the sender's membership of the guild.
	Member *Member `json:"member,omitempty"`
	// Ark is the ark card.
	Ark *MessageArk `json:"ark,omitempty"`
	// Seq orders messages within one channel. The documentation marks it as
	// deprecated after 2022-08-01.
	Seq int `json:"seq,omitempty"`
	// SeqInChannel orders messages within one channel; prefer it over Seq.
	SeqInChannel string `json:"seq_in_channel,omitempty"`
	// MessageReference quotes another message.
	MessageReference *MessageReference `json:"message_reference,omitempty"`
}

// MessageDelete is the body of the message delete events.
type MessageDelete struct {
	// Message is the deleted message.
	Message *GuildMessage `json:"message,omitempty"`
	// OpUser is the user who deleted it.
	OpUser *User `json:"op_user,omitempty"`
}

// MessageAudited is the body of MESSAGE_AUDIT_PASS and MESSAGE_AUDIT_REJECT.
type MessageAudited struct {
	// AuditID is the audit id.
	AuditID string `json:"audit_id,omitempty"`
	// MessageID is the message id, present only on an approval.
	MessageID string `json:"message_id,omitempty"`
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AuditTime is when the audit finished, in ISO8601.
	AuditTime string `json:"audit_time,omitempty"`
	// CreateTime is when the message was created, in ISO8601.
	CreateTime string `json:"create_time,omitempty"`
	// SeqInChannel orders messages within one channel.
	SeqInChannel string `json:"seq_in_channel,omitempty"`
}

// Emoji is the reaction emoji.
type Emoji struct {
	// ID is the emoji id: a number for a built-in emoji, or the emoji itself
	// for a Unicode emoji.
	ID string `json:"id,omitempty"`
	// Type is EmojiTypeSystem or EmojiTypeEmoji.
	Type int `json:"type,omitempty"`
}

// ReactionTarget is what a reaction was added to.
type ReactionTarget struct {
	// ID is the target id.
	ID string `json:"id,omitempty"`
	// Type is one of the ReactionTarget constants.
	Type int `json:"type,omitempty"`
}

// MessageReaction is the body of MESSAGE_REACTION_ADD and
// MESSAGE_REACTION_REMOVE.
type MessageReaction struct {
	// UserID is the user who reacted.
	UserID string `json:"user_id,omitempty"`
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// Target is what was reacted to.
	Target *ReactionTarget `json:"target,omitempty"`
	// Emoji is the emoji used.
	Emoji *Emoji `json:"emoji,omitempty"`
}

// IsBuiltinEmoji reports whether the reaction used a QQ built-in emoji.
func (e *Emoji) IsBuiltinEmoji() bool {
	return e != nil && e.Type == EmojiTypeSystem
}
