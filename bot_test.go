package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// TestGetBotInfoMatchesDocumentedExample reproduces the documented response.
func TestGetBotInfoMatchesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"id": "5777414462219517083",
		"username": "阳光小助手",
		"avatar": "https://thirdqq.qlogo.cn/g?a=1",
		"bot": true,
		"union_openid": "9F2E872045CCCC5948BEAF5B5FCCDF22",
		"union_user_account": "",
		"share_url": "https://qun.qq.com/qunpro/robot/qunshare?robot_uin=3889007780",
		"welcome_msg": "欢迎加入我们的群聊"
	}`)

	info, err := client.GetBotInfo(t.Context())
	if err != nil {
		t.Fatalf("GetBotInfo: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if req.Path != "/users/@me" {
		t.Errorf("path = %s, want /users/@me", req.Path)
	}

	if info.ID != "5777414462219517083" {
		t.Errorf("ID = %q", info.ID)
	}
	if info.Username != "阳光小助手" {
		t.Errorf("Username = %q", info.Username)
	}
	if !info.Bot {
		t.Error("Bot = false, want true")
	}
	if info.UnionOpenID != "9F2E872045CCCC5948BEAF5B5FCCDF22" {
		t.Errorf("UnionOpenID = %q", info.UnionOpenID)
	}
	if info.ShareURL == "" {
		t.Error("ShareURL must be decoded when the platform returns it")
	}
	if info.WelcomeMsg != "欢迎加入我们的群聊" {
		t.Errorf("WelcomeMsg = %q", info.WelcomeMsg)
	}
}

// TestGetBotInfoWithoutOptionalFields checks that the restricted fields being
// absent is not an error.
func TestGetBotInfoWithoutOptionalFields(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK,
		`{"id":"1","username":"bot","avatar":"","bot":true}`)

	info, err := client.GetBotInfo(t.Context())
	if err != nil {
		t.Fatalf("GetBotInfo: %v", err)
	}
	if info.UnionOpenID != "" || info.UnionUserAccount != "" {
		t.Errorf("info = %+v, want the restricted fields empty", info)
	}
}

// TestGetJoinedGuildsWrappedForm covers the shape the field table documents.
func TestGetJoinedGuildsWrappedForm(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"guilds":[
		{"id":"2452178231489345741","name":"读书分享会","owner":false,
		 "owner_id":"17481532452010052342","joined_at":"2025-01-09T15:17:23+08:00",
		 "member_count":6,"max_members":5000000,"description":"一起读书，共同成长"}
	]}`)

	guilds, err := client.GetJoinedGuilds(t.Context(), "", "", 20)
	if err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}

	req := last(t, captured)
	if req.Path != "/users/@me/guilds" {
		t.Errorf("path = %s, want /users/@me/guilds", req.Path)
	}
	if req.Query != "limit=20" {
		t.Errorf("query = %q, want limit=20", req.Query)
	}

	if len(guilds) != 1 {
		t.Fatalf("guilds = %d, want 1", len(guilds))
	}
	got := guilds[0]
	if got.ID != "2452178231489345741" || got.Name != "读书分享会" {
		t.Errorf("guild = %+v", got)
	}
	if got.OwnerID != "17481532452010052342" || got.Owner {
		t.Errorf("guild = %+v, want the owner fields decoded", got)
	}
	if got.MemberCount != 6 || got.MaxMembers != 5000000 {
		t.Errorf("guild = %+v, want the member counts", got)
	}
	if got.JoinedAt != "2025-01-09T15:17:23+08:00" {
		t.Errorf("JoinedAt = %q", got.JoinedAt)
	}
	if got.Description != "一起读书，共同成长" {
		t.Errorf("Description = %q", got.Description)
	}
}

// TestGetJoinedGuildsBareArrayForm covers the shape the response example shows,
// which differs from the field table. Both must decode.
func TestGetJoinedGuildsBareArrayForm(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `[
		{"id":"G1","name":"读书分享会","owner":false,"member_count":6},
		{"id":"G2","name":"英语学习角","owner":true,"member_count":35}
	]`)

	guilds, err := client.GetJoinedGuilds(t.Context(), "", "", 0)
	if err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	if len(guilds) != 2 {
		t.Fatalf("guilds = %d, want 2 from the bare array form", len(guilds))
	}
	if guilds[0].ID != "G1" || guilds[1].Name != "英语学习角" {
		t.Errorf("guilds = %+v", guilds)
	}
}

// TestGetJoinedGuildsPaginationQuery checks the documented cursor parameters.
func TestGetJoinedGuildsPaginationQuery(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"guilds":[]}`)

	if _, err := client.GetJoinedGuilds(t.Context(), "AFTER", "BEFORE", 0); err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	query := last(t, captured).Query
	// url.Values sorts keys alphabetically: after, before.
	if query != "after=AFTER&before=BEFORE" {
		t.Errorf("query = %q, want both cursors", query)
	}

	// A single cursor must be sent on its own.
	if _, err := client.GetJoinedGuilds(t.Context(), "", "BEFORE", 0); err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	if query := last(t, captured).Query; query != "before=BEFORE" {
		t.Errorf("query = %q, want only before", query)
	}

	// No parameters at all must leave the address bare.
	if _, err := client.GetJoinedGuilds(t.Context(), "", "", 0); err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	if query := last(t, captured).Query; query != "" {
		t.Errorf("query = %q, want none", query)
	}
}

// TestGetJoinedGuildsCapsLimit checks the documented maximum of 100.
func TestGetJoinedGuildsCapsLimit(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"guilds":[]}`)

	if _, err := client.GetJoinedGuilds(t.Context(), "", "", 5000); err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	if query := last(t, captured).Query; query != "limit=100" {
		t.Errorf("query = %q, want the limit capped at 100", query)
	}
}

// TestBotEndpointErrors checks that a platform failure is surfaced.
func TestBotEndpointErrors(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":11251,"message":"appid 错误"}`)

	if _, err := client.GetBotInfo(t.Context()); err == nil {
		t.Error("GetBotInfo must surface a platform failure")
	}
	if _, err := client.GetJoinedGuilds(t.Context(), "", "", 0); err == nil {
		t.Error("GetJoinedGuilds must surface a platform failure")
	}
}

// TestGetJoinedGuildsEmptyBodyIsNotAnError covers a body the platform may omit.
func TestGetJoinedGuildsEmptyBody(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusNoContent, "")

	guilds, err := client.GetJoinedGuilds(t.Context(), "", "", 0)
	if err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	if len(guilds) != 0 {
		t.Errorf("guilds = %+v, want none", guilds)
	}
}

// TestGetJoinedGuildsRejectsMalformedBody checks that a non-list, non-object
// body is reported instead of silently yielding nothing.
func TestGetJoinedGuildsRejectsMalformedBody(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `"just-a-string"`)

	if _, err := client.GetJoinedGuilds(t.Context(), "", "", 0); err == nil {
		t.Error("a malformed body must be reported")
	}
}

// TestUsersMeAddressIsNotEscaped guards the @me literal, which must reach the
// platform verbatim.
func TestUsersMeAddressIsNotEscaped(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"id":"1"}`)

	if _, err := client.GetBotInfo(t.Context()); err != nil {
		t.Fatalf("GetBotInfo: %v", err)
	}
	req := last(t, captured)
	if !strings.Contains(req.Path, "@me") {
		t.Errorf("path = %s, want the @me literal", req.Path)
	}
}
