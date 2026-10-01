package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// TestAddReactionMatchesDocumentedRequest reproduces the documented example.
func TestAddReactionMatchesDocumentedRequest(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusNoContent, "")

	err := client.AddReaction(t.Context(), "1013531",
		"08c095b7ba8ed4abd7e00110cbd83f3841489aa2bd9006", EmojiTypeSystem, "203")
	if err != nil {
		t.Fatalf("AddReaction: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", req.Method)
	}
	want := "/channels/1013531/messages/08c095b7ba8ed4abd7e00110cbd83f3841489aa2bd9006/reactions/1/203"
	if req.Path != want {
		t.Errorf("path = %s\nwant       %s", req.Path, want)
	}
	if len(req.Body) != 0 {
		t.Errorf("body = %s, want no body", req.Body)
	}
}

func TestRemoveReactionIsDelete(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusNoContent, "")

	if err := client.RemoveReaction(t.Context(), "CH", "MSG", EmojiTypeSystem, "4"); err != nil {
		t.Fatalf("RemoveReaction: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", req.Method)
	}
	if req.Path != "/channels/CH/messages/MSG/reactions/1/4" {
		t.Errorf("path = %s", req.Path)
	}
}

// TestReactionUsersParsesPage covers the documented list response.
func TestReactionUsersParsesPage(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"users":[
			{"id":"1158788878435714165","username":"频道机器人","avatar":"http://example.com/a.png"}
		],
		"cookie":"1_2",
		"is_end":false
	}`)

	page, err := client.ReactionUsers(t.Context(), "CH", "MSG", EmojiTypeSystem, "203", "", 20)
	if err != nil {
		t.Fatalf("ReactionUsers: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if req.Query != "limit=20" {
		t.Errorf("query = %q, want limit=20 and no empty cookie", req.Query)
	}

	if len(page.Users) != 1 {
		t.Fatalf("Users = %d, want 1", len(page.Users))
	}
	if page.Users[0].ID != "1158788878435714165" || page.Users[0].Username != "频道机器人" {
		t.Errorf("user = %+v", page.Users[0])
	}
	if page.Cookie != "1_2" {
		t.Errorf("Cookie = %q, want the pagination cookie", page.Cookie)
	}
	if page.IsEnd {
		t.Error("IsEnd = true, want false for a non-final page")
	}
}

// TestReactionUsersPaginationQuery checks that a cookie is passed through and
// that no limit is sent on a follow-up page.
func TestReactionUsersPaginationQuery(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"users":[],"cookie":"","is_end":true}`)

	page, err := client.ReactionUsers(t.Context(), "CH", "MSG", EmojiTypeEmoji, "🌹", "1_2", 0)
	if err != nil {
		t.Fatalf("ReactionUsers: %v", err)
	}
	if !page.IsEnd {
		t.Error("IsEnd = false, want true")
	}

	req := last(t, captured)
	if req.Query != "cookie=1_2" {
		t.Errorf("query = %q, want cookie=1_2 only", req.Query)
	}
}

// TestReactionEscapesEmojiID checks that a Unicode emoji id cannot break the
// route, since EmojiTypeEmoji uses the emoji itself as the id.
func TestReactionEscapesEmojiID(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusNoContent, "")

	if err := client.AddReaction(t.Context(), "CH", "MSG", EmojiTypeEmoji, "🌹/x"); err != nil {
		t.Fatalf("AddReaction: %v", err)
	}

	req := last(t, captured)
	if strings.Contains(req.EscapedPath, "🌹") || !strings.Contains(req.EscapedPath, "%2F") {
		t.Errorf("escaped path = %s, want the emoji encoded", req.EscapedPath)
	}
}

func TestReactionRejectsBadInput(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusNoContent, "")

	cases := map[string]func() error{
		"no channel": func() error { return client.AddReaction(t.Context(), "", "MSG", EmojiTypeSystem, "1") },
		"no message": func() error { return client.AddReaction(t.Context(), "CH", "", EmojiTypeSystem, "1") },
		"no emoji id": func() error {
			return client.RemoveReaction(t.Context(), "CH", "MSG", EmojiTypeSystem, "")
		},
		"bad emoji type": func() error { return client.AddReaction(t.Context(), "CH", "MSG", 7, "1") },
		"users no channel": func() error {
			_, err := client.ReactionUsers(t.Context(), "", "MSG", EmojiTypeSystem, "1", "", 0)
			return err
		},
		"users bad type": func() error {
			_, err := client.ReactionUsers(t.Context(), "CH", "MSG", 0, "1", "", 0)
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("invalid input must be rejected before any request")
			}
		})
	}
	if len(*captured) != 0 {
		t.Errorf("a rejected call still sent %d requests", len(*captured))
	}
}

func TestEmojiTypeConstants(t *testing.T) {
	if EmojiTypeSystem != 1 || EmojiTypeEmoji != 2 {
		t.Errorf("emoji types = %d/%d, want 1/2", EmojiTypeSystem, EmojiTypeEmoji)
	}
}

// TestReactionEmojiGuessesTheType covers the helper that picks the emoji type
// from the id shape.
func TestReactionEmojiGuessesTheType(t *testing.T) {
	emojiType, id, ok := ReactionEmoji("203")
	if !ok || emojiType != EmojiTypeSystem || id != "203" {
		t.Errorf("ReactionEmoji(203) = %d, %q, %v; want the system type", emojiType, id, ok)
	}

	emojiType, id, ok = ReactionEmoji("🌹")
	if !ok || emojiType != EmojiTypeEmoji || id != "🌹" {
		t.Errorf("ReactionEmoji(rose) = %d, %q, %v; want the unicode type", emojiType, id, ok)
	}

	if _, _, ok := ReactionEmoji(""); ok {
		t.Error("an empty id must not be accepted")
	}
	if _, _, ok := ReactionEmoji("   "); ok {
		t.Error("a blank id must not be accepted")
	}
}

// TestReactionPropagatesErrors checks that a platform failure is surfaced.
func TestReactionPropagatesErrors(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":11282,"message":"检查是否是管理员未通过"}`)

	if err := client.AddReaction(t.Context(), "CH", "MSG", EmojiTypeSystem, "1"); err == nil {
		t.Error("AddReaction must surface a platform failure")
	}
	if _, err := client.ReactionUsers(t.Context(), "CH", "MSG", EmojiTypeSystem, "1", "", 0); err == nil {
		t.Error("ReactionUsers must surface a platform failure")
	}
}
