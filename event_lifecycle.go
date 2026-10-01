package qqbotsdk

// ReadyData is the body of READY, sent once a websocket session is
// authenticated.
//
// READY is delivered on the connection itself rather than through an intent, so
// it arrives whichever intents were requested.
type ReadyData struct {
	// Version is the gateway protocol version.
	Version int `json:"version,omitempty"`
	// SessionID identifies the session, and is what a Resume needs.
	SessionID string `json:"session_id,omitempty"`
	// User is the bot account.
	User *User `json:"user,omitempty"`
	// Shard is the documented two element array, [shard id, shard count].
	Shard []int `json:"shard,omitempty"`
}

// ShardInfo returns the shard pair as a Shard, reporting false when the array
// is absent or malformed.
func (r *ReadyData) ShardInfo() (Shard, bool) {
	if r == nil || len(r.Shard) != 2 {
		return Shard{}, false
	}
	return Shard{ID: r.Shard[0], Count: r.Shard[1]}, true
}

// ResumedData is the body of RESUMED.
//
// The documented example shows the body as an empty string rather than an
// object, so there is nothing to decode. This type exists so that the event
// still has an entry in the decode table.
type ResumedData struct{}

// UnmarshalJSON accepts the empty string, and any other shape, since the
// platform sends no usable body here.
func (*ResumedData) UnmarshalJSON([]byte) error { return nil }
