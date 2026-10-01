package qqbotsdk

// AudioAction is the body of the four audio events: AUDIO_START, AUDIO_FINISH,
// AUDIO_ON_MIC and AUDIO_OFF_MIC, which report playback and microphone changes
// in a voice channel.
//
// Note on the source: the intent table lists those four event names without
// describing their body. The audio object page defines AudioAction, carrying
// the guild and channel ids an event payload has, and no documented endpoint
// accepts it — the playback endpoint takes AudioControl instead, and the
// microphone endpoints take an empty body. AudioAction is therefore the event
// body, and these four events share it.
type AudioAction struct {
	// GuildID is the guild id.
	GuildID string `json:"guild_id,omitempty"`
	// ChannelID is the voice channel id.
	ChannelID string `json:"channel_id,omitempty"`
	// AudioURL is the audio being played, for the playback events.
	AudioURL string `json:"audio_url,omitempty"`
	// Text describes the audio, such as "简单爱-周杰伦".
	Text string `json:"text,omitempty"`
}
