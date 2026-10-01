package qqbotsdk

import (
	"context"
	"net/http"
)

// Gateway endpoints.
const (
	gatewayPath    = "/gateway"
	gatewayBotPath = "/gateway/bot"
)

// Gateway is the response of GET /gateway.
type Gateway struct {
	// URL is the WSS address to connect to.
	URL string `json:"url"`
}

// SessionStartLimit describes how many sessions may still be created.
type SessionStartLimit struct {
	// Total is the number of sessions creatable per 24 hours.
	Total int `json:"total"`
	// Remaining is how many may still be created.
	Remaining int `json:"remaining"`
	// ResetAfter is the time until the counter resets, in milliseconds.
	ResetAfter int `json:"reset_after"`
	// MaxConcurrency is how many sessions may be created per 5 seconds.
	MaxConcurrency int `json:"max_concurrency"`
}

// GatewayBot is the response of GET /gateway/bot, which additionally reports
// the recommended shard count and the session creation budget.
type GatewayBot struct {
	// URL is the WSS address to connect to.
	URL string `json:"url"`
	// Shards is the recommended number of shards.
	Shards int `json:"shards"`
	// SessionStartLimit reports the session creation budget.
	SessionStartLimit SessionStartLimit `json:"session_start_limit"`
}

// GetGateway returns the general WSS access point.
//
// The endpoint is heavily rate limited: the documentation gives 2 requests per
// minute with a burst of 10. A caller that opens a connection per event, or
// that polls this from several goroutines, is therefore answered with
// "接口调用超过频率限制" (40023001). Fetch the address once and reuse it; a
// reconnect reuses the same address.
func (c *Client) GetGateway(ctx context.Context) (*Gateway, error) {
	var gateway Gateway
	if err := c.doJSON(ctx, http.MethodGet, gatewayPath, nil, &gateway, openAPICall); err != nil {
		return nil, err
	}
	return &gateway, nil
}

// GetGatewayBot returns the WSS access point together with the recommended
// shard count and the session creation limit.
func (c *Client) GetGatewayBot(ctx context.Context) (*GatewayBot, error) {
	var gateway GatewayBot
	if err := c.doJSON(ctx, http.MethodGet, gatewayBotPath, nil, &gateway, openAPICall); err != nil {
		return nil, err
	}
	return &gateway, nil
}

// ShardID computes the shard that receives events for a guild.
//
// The documented rule is shard_id = (guild_id >> 22) % num_shards.
func ShardID(guildID uint64, numShards int) int {
	if numShards <= 0 {
		return 0
	}
	return int((guildID >> 22) % uint64(numShards))
}
