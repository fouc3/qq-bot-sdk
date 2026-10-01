//go:build production

// Package qqbotsdk_test carries the test that runs against the real QQ Bot
// platform and a real OneBot client.
//
// It sits behind the production build tag so an ordinary `go test` never
// reaches the network or sends a real message:
//
//	QQBOT_APPID=...      QQBOT_SECRET=...    QQBOT_BOT_QQ=...
//	ONEBOT_URL=...       ONEBOT_KEY=...      ONEBOT_SELF_QQ=...
//	go test -tags production -run TestProduction -v .
//
// The OneBot client controls a real QQ account, and the bot number is what that
// account messages. The round trip test therefore proves both directions: the
// platform delivers the user's message to this SDK, and the platform accepts
// the reply this SDK sends back.
package qqbotsdk_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

// productionConfig is everything the live test needs, read from the
// environment so no credential is ever written to the repository.
type productionConfig struct {
	appID     string
	secret    string
	botQQ     string
	oneBotURL string
	oneBotKey string
	selfQQ    string
}

// loadProductionConfig reads the environment, skipping the test when it is
// incomplete rather than failing on a machine that is not set up for it.
func loadProductionConfig(t *testing.T) productionConfig {
	t.Helper()
	cfg := productionConfig{
		appID:     os.Getenv("QQBOT_APPID"),
		secret:    os.Getenv("QQBOT_SECRET"),
		botQQ:     os.Getenv("QQBOT_BOT_QQ"),
		oneBotURL: strings.TrimRight(os.Getenv("ONEBOT_URL"), "/"),
		oneBotKey: os.Getenv("ONEBOT_KEY"),
		selfQQ:    os.Getenv("ONEBOT_SELF_QQ"),
	}
	missing := make([]string, 0, 6)
	for name, value := range map[string]string{
		"QQBOT_APPID":    cfg.appID,
		"QQBOT_SECRET":   cfg.secret,
		"QQBOT_BOT_QQ":   cfg.botQQ,
		"ONEBOT_URL":     cfg.oneBotURL,
		"ONEBOT_KEY":     cfg.oneBotKey,
		"ONEBOT_SELF_QQ": cfg.selfQQ,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Skipf("production test needs: %s", strings.Join(missing, ", "))
	}
	return cfg
}

// productionClient builds a client from the live credentials.
func productionClient(cfg productionConfig) *qqbotsdk.Client {
	return qqbotsdk.NewClient(cfg.appID, cfg.secret)
}

// TestProductionBotIdentity checks the credentials and the request plumbing
// against the real API, which is the cheapest thing that can go wrong.
func TestProductionBotIdentity(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	token, err := client.AccessToken(ctx)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token.Value == "" {
		t.Fatal("the platform returned an empty access token")
	}
	t.Logf("access token acquired, %d chars, expires at %s",
		len(token.Value), token.ExpiresAt.Format(time.RFC3339))

	info, err := client.GetBotInfo(ctx)
	if err != nil {
		t.Fatalf("GetBotInfo: %v", err)
	}
	if info.ID == "" {
		t.Fatal("the platform returned no bot id")
	}
	// The live platform omits the bot key that the documented example shows,
	// so this reports whether it was sent instead of asserting a value.
	value, reported := info.IsBot()
	t.Logf("bot identity: id=%s username=%q avatar=%q bot=%v (reported=%v) share_url=%q union_openid=%q",
		info.ID, info.Username, info.Avatar, value, reported, info.ShareURL, info.UnionOpenID)
	if reported && !value {
		t.Error("the platform explicitly reports this bot account as not a bot")
	}
}

// TestProductionC2CRoundTrip drives the whole path: a real user sends the bot a
// single chat message through OneBot, the SDK receives it over the gateway
// websocket, and the SDK replies with a passive message the platform accepts.
//
// Sending requires the bot to be online on the websocket, so the connection is
// established before anything is sent.
func TestProductionC2CRoundTrip(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	gateway, err := client.GetGateway(ctx)
	if err != nil {
		t.Fatalf("GetGateway: %v", err)
	}
	t.Logf("gateway: %s", gateway.URL)

	// One handler for the readiness signal, one for the message we are waiting
	// for. Both decode through the typed event bodies.
	ready := make(chan struct{}, 1)
	messages := make(chan *qqbotsdk.C2CMessageCreateData, 8)

	client.RegisterFunc(qqbotsdk.EventReady, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			return err
		}
		data := value.(*qqbotsdk.ReadyData)
		shard, _ := data.ShardInfo()
		t.Logf("READY: session=%s version=%d shard=%+v", data.SessionID, data.Version, shard)
		select {
		case ready <- struct{}{}:
		default:
		}
		return nil
	})

	client.RegisterFunc(qqbotsdk.EventC2CMessageCreate, func(_ context.Context, event *qqbotsdk.Event) error {
		value, err := event.Decode()
		if err != nil {
			t.Errorf("decoding C2C_MESSAGE_CREATE: %v", err)
			return err
		}
		data := value.(*qqbotsdk.C2CMessageCreateData)
		t.Logf("event C2C_MESSAGE_CREATE: id=%s content=%q type=%d openid=%s",
			data.ID, data.Content, data.MessageType, data.Author.UserOpenID)
		select {
		case messages <- data:
		default:
		}
		return nil
	})

	client.UseTransport(qqbotsdk.NewWebSocketTransport(gateway.URL,
		qqbotsdk.WithIntents(qqbotsdk.IntentGroupAndC2CEvent)))

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Errorf("Stop: %v", err)
		}
	})

	select {
	case <-ready:
	case <-time.After(45 * time.Second):
		t.Fatal("the websocket never became ready")
	}

	// Have the controlled account message the bot, tagged so an unrelated
	// message cannot satisfy the test.
	probe := fmt.Sprintf("PING-%d", time.Now().UnixNano())
	sent := oneBotSendPrivate(t, cfg, cfg.botQQ, probe)
	t.Logf("the test account sent %q to the bot (OneBot message id %d)", probe, sent)

	var received *qqbotsdk.C2CMessageCreateData
	deadline := time.After(90 * time.Second)
	for received == nil {
		select {
		case data := <-messages:
			if strings.Contains(data.Content, probe) {
				received = data
			} else {
				t.Logf("ignoring an unrelated message: %q", data.Content)
			}
		case <-deadline:
			t.Fatal("the bot never received the probe message")
		}
	}

	if received.Author == nil || received.Author.UserOpenID == "" {
		t.Fatalf("the event carries no user openid: %+v", received.Author)
	}
	if received.MessageScene == nil {
		t.Error("the event carries no message scene")
	} else if idx, ok := received.MessageScene.MsgIdx(); ok {
		t.Logf("message index for de-duplication: %s", idx)
	}

	// A passive reply carries the message id from the event body, which the
	// documentation says is valid for five minutes.
	reply := &qqbotsdk.Message{
		Content: "PONG " + probe,
		MsgID:   received.ID,
		MsgSeq:  1,
	}
	response, err := client.SendC2CMessage(ctx, received.Author.UserOpenID, reply)
	if err != nil {
		t.Fatalf("SendC2CMessage: %v", err)
	}
	if response.ID == "" {
		t.Error("the platform accepted the reply but returned no message id")
	}
	t.Logf("reply accepted: id=%s timestamp=%s", response.ID, response.Timestamp)

	// Best effort: confirm the reply actually reached the controlled account.
	verifyReplyArrived(t, cfg, cfg.botQQ, "PONG "+probe)
}

// oneBotSendPrivate sends a private message through the OneBot HTTP API and
// returns the message id it reports.
func oneBotSendPrivate(t *testing.T, cfg productionConfig, userID, text string) int64 {
	t.Helper()
	data := oneBotCall(t, cfg, "send_private_msg", map[string]any{
		"user_id": json.Number(userID),
		"message": text,
	})
	id, _ := data["message_id"].(float64)
	if id == 0 {
		t.Fatalf("OneBot did not report a message id: %v", data)
	}
	return int64(id)
}

// oneBotCall performs one OneBot action and returns its data object.
func oneBotCall(t *testing.T, cfg productionConfig, action string, payload any) map[string]any {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encoding the %s payload: %v", action, err)
	}
	req, err := http.NewRequest(http.MethodPost, cfg.oneBotURL+"/"+action, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("building the %s request: %v", action, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.oneBotKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("calling OneBot %s: %v", action, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatalf("reading the %s response: %v", action, err)
	}
	var envelope struct {
		Status  string         `json:"status"`
		RetCode int            `json:"retcode"`
		Data    map[string]any `json:"data"`
		Wording string         `json:"wording"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decoding the %s response %s: %v", action, raw, err)
	}
	if envelope.Status != "ok" {
		t.Fatalf("OneBot %s failed: retcode=%d wording=%s", action, envelope.RetCode, envelope.Wording)
	}
	return envelope.Data
}

// verifyReplyArrived asks OneBot for the recent history with the bot and looks
// for the reply text.
//
// Not every OneBot build exposes a working history action, so a refusal is
// reported rather than treated as a failure of the SDK: the platform's accepted
// response is the evidence for the send path, and this only strengthens it when
// the client can answer.
func verifyReplyArrived(t *testing.T, cfg productionConfig, userID, want string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		found, ok := oneBotHistoryContains(t, cfg, userID, want)
		if !ok {
			t.Log("OneBot cannot report this conversation's history, " +
				"so delivery to the account is unconfirmed by the client")
			return
		}
		if found {
			t.Logf("confirmed: the reply %q reached the controlled account", want)
			return
		}
		time.Sleep(3 * time.Second)
	}
	t.Errorf("the reply %q never appeared in the controlled account's history", want)
}

// oneBotHistoryContains reports whether the history holds want, and whether the
// history could be read at all.
func oneBotHistoryContains(t *testing.T, cfg productionConfig, userID, want string) (found, readable bool) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"user_id": json.Number(userID), "count": 20})
	if err != nil {
		return false, false
	}
	req, err := http.NewRequest(http.MethodPost,
		cfg.oneBotURL+"/get_friend_msg_history", bytes.NewReader(body))
	if err != nil {
		return false, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.oneBotKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return false, false
	}
	var envelope struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Status != "ok" {
		return false, false
	}
	return strings.Contains(string(raw), want), true
}

// TestProductionReadOnlyEndpoints exercises the remaining read-only calls
// against the live platform, which validates their request paths, query
// building and response decoding without changing anything.
func TestProductionReadOnlyEndpoints(t *testing.T) {
	cfg := loadProductionConfig(t)
	client := productionClient(cfg)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	botGateway, err := client.GetGatewayBot(ctx)
	if err != nil {
		t.Fatalf("GetGatewayBot: %v", err)
	}
	t.Logf("gateway/bot: url=%s shards=%d session_limit={total:%d remaining:%d max_concurrency:%d}",
		botGateway.URL, botGateway.Shards, botGateway.SessionStartLimit.Total,
		botGateway.SessionStartLimit.Remaining, botGateway.SessionStartLimit.MaxConcurrency)
	if botGateway.URL == "" {
		t.Error("gateway/bot returned no url")
	}

	// The guild list shape is the one the documentation contradicts itself on,
	// so a live call is the only way to know which form the platform sends.
	guilds, err := client.GetJoinedGuilds(ctx, "", "", 20)
	if err != nil {
		t.Fatalf("GetJoinedGuilds: %v", err)
	}
	t.Logf("joined guilds: %d", len(guilds))
	for i, guild := range guilds {
		if i >= 3 {
			t.Logf("  ... and %d more", len(guilds)-i)
			break
		}
		t.Logf("  guild id=%s name=%q members=%d owner=%v",
			guild.ID, guild.Name, guild.MemberCount, guild.Owner)
	}

	// A menu that was never set must come back without a menu, not as an error.
	menu, err := client.GetMenu(ctx)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	if menu.Menu == nil {
		t.Logf("menu: version=%d, not configured", menu.Version)
	} else {
		t.Logf("menu: version=%d, %d items", menu.Version, len(menu.Menu.Items))
	}

	panelPage, err := client.ListPanels(ctx, qqbotsdk.PanelScopeC2C, "", 10)
	if err != nil {
		t.Fatalf("ListPanels: %v", err)
	}
	t.Logf("c2c panels: %d records, next_cursor=%q is_end=%v",
		len(panelPage.Records), panelPage.NextCursor, panelPage.IsEnd)
	for _, record := range panelPage.Records {
		t.Logf("  panel id=%s scope=%s target=%s", record.PanelID, record.Scope, record.TargetType)
	}
}
