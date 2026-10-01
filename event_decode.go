package qqbotsdk

import "fmt"

// EventDataFor returns a new, empty value of the type that carries the body of
// the given event type, or nil when the event has no structure defined yet.
//
// The returned value is a pointer, ready to be filled by Payload.DecodeData or
// DecodeEvent.
func EventDataFor(eventType string) any {
	switch eventType {
	// GUILDS.
	case EventGuildCreate, EventGuildUpdate, EventGuildDelete:
		return &GuildInfo{}
	case EventChannelCreate, EventChannelUpdate, EventChannelDelete:
		return &ChannelInfo{}

	// GUILD_MEMBERS.
	case EventGuildMemberAdd, EventGuildMemberUpdate, EventGuildMemberRemove:
		return &MemberWithGuildID{}

	// Channel messages, from any of the message intents.
	case EventMessageCreate, EventAtMessageCreate, EventDirectMessageCreate:
		return &GuildMessage{}
	case EventMessageDelete, EventPublicMessageDelete, EventDirectMessageDelete:
		return &MessageDelete{}
	case EventMessageAuditPass, EventMessageAuditReject:
		return &MessageAudited{}

	// GUILD_MESSAGE_REACTIONS.
	case EventMessageReactionAdd, EventMessageReactionRemove:
		return &MessageReaction{}

	// FORUM_EVENT.
	case EventForumThreadCreate, EventForumThreadUpdate, EventForumThreadDelete:
		return &ForumThreadEvent{}
	case EventForumPostCreate, EventForumPostDelete:
		return &ForumPostEvent{}
	case EventForumReplyCreate, EventForumReplyDelete:
		return &ForumReplyEvent{}
	case EventForumPublishAuditResult:
		return &ForumAuditResult{}

	// AUDIO_ACTION.
	case EventAudioStart, EventAudioFinish, EventAudioOnMic, EventAudioOffMic:
		return &AudioAction{}

	// Connection lifecycle, delivered without an intent.
	case EventReady:
		return &ReadyData{}
	case EventResumed:
		return &ResumedData{}

	// GROUP_AND_C2C.
	case EventC2CMessageCreate:
		return &C2CMessageCreateData{}
	case EventGroupAtMessageCreate, EventGroupMessageCreate:
		return &GroupMessageCreateData{}
	case EventC2CMsgReceive:
		return &C2CMsgReceiveData{}
	case EventC2CMsgReject:
		return &C2CMsgRejectData{}
	case EventFriendAdd:
		return &FriendAddData{}
	case EventFriendDel:
		return &FriendDelData{}
	case EventGroupAddRobot:
		return &GroupAddRobotData{}
	case EventGroupDelRobot:
		return &GroupDelRobotData{}
	case EventGroupMsgReceive:
		return &GroupMsgReceiveData{}
	case EventGroupMsgReject:
		return &GroupMsgRejectData{}
	case EventSubscribeMsgStatus:
		return &SubscribeMessageStatusData{}

	// GROUP_MEMBER.
	case EventGroupMemberAdd:
		return &GroupMemberAddData{}
	case EventGroupMemberRemove:
		return &GroupMemberRemoveData{}
	case EventGroupJoinRequest:
		return &GroupJoinRequestData{}

	// INTERACTION.
	case EventInteractionCreate:
		return &InteractionCreateData{}
	}
	return nil
}

// DecodeEvent decodes the body of a dispatch payload into the structure that
// matches its event type.
//
// It returns the decoded value, whose concrete type is the one EventDataFor
// reports for that event type:
//
//	value, err := qqbotsdk.DecodeEvent(payload)
//	switch data := value.(type) {
//	case *qqbotsdk.GroupMessageCreateData:
//		fmt.Println(data.Content)
//	}
//
// An unknown event type, and one whose structure is not defined yet, are both
// reported as an error rather than silently decoded into nothing.
func DecodeEvent(p *Payload) (any, error) {
	if p == nil {
		return nil, fmt.Errorf("qqbotsdk: payload is nil")
	}
	if p.Op != OpDispatch {
		return nil, fmt.Errorf("qqbotsdk: opcode %s carries no event body", p.Op)
	}
	target := EventDataFor(p.Type)
	if target == nil {
		return nil, fmt.Errorf("qqbotsdk: no structure is defined for event type %q", p.Type)
	}
	if err := p.DecodeData(target); err != nil {
		return nil, err
	}
	return target, nil
}
