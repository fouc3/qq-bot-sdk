package qqbotsdk

import (
	"testing"
)

// TestDecodeReady reproduces the documented READY payload.
func TestDecodeReady(t *testing.T) {
	payload := &Payload{
		Op:   OpDispatch,
		Seq:  intPtr(1),
		Type: EventReady,
		Data: []byte(`{
			"version": 1,
			"session_id": "082ee18c-0be3-491b-9d8b-fbd95c51673a",
			"user": {"id": "6158788878435714165", "username": "群pro测试机器人", "bot": true},
			"shard": [0, 0]
		}`),
	}

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	data, ok := value.(*ReadyData)
	if !ok {
		t.Fatalf("value = %T, want *ReadyData", value)
	}

	if data.Version != 1 {
		t.Errorf("Version = %d, want 1", data.Version)
	}
	if data.SessionID != "082ee18c-0be3-491b-9d8b-fbd95c51673a" {
		t.Errorf("SessionID = %q", data.SessionID)
	}
	if data.User == nil || !data.User.Bot || data.User.Username != "群pro测试机器人" {
		t.Errorf("User = %+v", data.User)
	}

	shard, ok := data.ShardInfo()
	if !ok {
		t.Fatal("ShardInfo reported no shard")
	}
	if shard.ID != 0 || shard.Count != 0 {
		t.Errorf("shard = %+v, want the documented pair", shard)
	}
}

// TestReadyShardInfo covers the documented two element array, including the
// shapes it must refuse.
func TestReadyShardInfo(t *testing.T) {
	shard, ok := (&ReadyData{Shard: []int{2, 4}}).ShardInfo()
	if !ok || shard.ID != 2 || shard.Count != 4 {
		t.Errorf("ShardInfo = %+v, %v", shard, ok)
	}

	for name, data := range map[string]*ReadyData{
		"nil":       nil,
		"no shard":  {},
		"one entry": {Shard: []int{1}},
		"three":     {Shard: []int{1, 2, 3}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := data.ShardInfo(); ok {
				t.Error("a shard that is not a two element array must be refused")
			}
		})
	}
}

// TestDecodeResumed covers the documented body, which is an empty string rather
// than an object.
func TestDecodeResumed(t *testing.T) {
	payload := &Payload{Op: OpDispatch, Type: EventResumed, Data: []byte(`""`)}

	value, err := DecodeEvent(payload)
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if _, ok := value.(*ResumedData); !ok {
		t.Fatalf("value = %T, want *ResumedData", value)
	}
}

// TestDecodeAudioEvents covers the four audio events, which share the
// documented audio action object.
func TestDecodeAudioEvents(t *testing.T) {
	body := `{
		"guild_id": "10001", "channel_id": "10002",
		"audio_url": "https://example.com/a.mp3", "text": "简单爱-周杰伦"
	}`

	for _, eventType := range []string{
		EventAudioStart, EventAudioFinish, EventAudioOnMic, EventAudioOffMic,
	} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*AudioAction)
			if data.GuildID != "10001" || data.ChannelID != "10002" {
				t.Errorf("data = %+v", data)
			}
			if data.AudioURL != "https://example.com/a.mp3" || data.Text != "简单爱-周杰伦" {
				t.Errorf("data = %+v", data)
			}
		})
	}
}

// TestDecodeGuildMemberEvents covers the three member events, which carry the
// documented member with its guild id.
func TestDecodeGuildMemberEvents(t *testing.T) {
	body := `{
		"guild_id": "G1",
		"user": {"id": "U1", "username": "abc"},
		"nick": "小明",
		"roles": ["1", "2"],
		"joined_at": "2021-04-12T16:34:42+08:00"
	}`

	for _, eventType := range []string{
		EventGuildMemberAdd, EventGuildMemberUpdate, EventGuildMemberRemove,
	} {
		t.Run(eventType, func(t *testing.T) {
			data := mustDecode(t, dispatchPayload(t, eventType, body)).(*MemberWithGuildID)
			if data.GuildID != "G1" {
				t.Errorf("GuildID = %q", data.GuildID)
			}
			if data.User == nil || data.User.ID != "U1" {
				t.Errorf("User = %+v", data.User)
			}
			if data.Nick != "小明" || len(data.Roles) != 2 {
				t.Errorf("data = %+v", data)
			}
			if data.JoinedAt != "2021-04-12T16:34:42+08:00" {
				t.Errorf("JoinedAt = %q", data.JoinedAt)
			}
		})
	}
}

// TestReadyAndResumedAreNotIntentBound checks that the lifecycle events map to
// no intent, since they are delivered on the connection itself.
func TestReadyAndResumedAreNotIntentBound(t *testing.T) {
	for _, eventType := range []string{EventReady, EventResumed} {
		if intent := IntentForEvent(eventType); intent != 0 {
			t.Errorf("IntentForEvent(%s) = %d, want no intent", eventType, intent)
		}
	}
}

// intPtr is a small helper for the optional sequence number.
func intPtr(v int64) *int64 { return &v }
