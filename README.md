# qq-bot-sdk

QQ 机器人（QQ Bot）开放平台 SDK，Go 实现。参考官方文档：[QQ 机器人开发文档 · API v2](https://bot.q.qq.com/wiki/develop/api-v2/)。

模块路径：`github.com/fouc3/qq-bot-sdk`

## 已实现

| 能力 | 说明 |
| --- | --- |
| `Client` | OpenAPI 客户端，默认地址与请求头取自文档，均可覆盖 |
| `GetAppAccessToken` | 调用 `POST /app/getAppAccessToken`，强制获取新凭证 |
| `AccessToken` | 带缓存的凭证获取，到期前自动刷新 |
| `InvalidateToken` | 主动作废缓存凭证 |
| `TokenSource` | 凭证来源接口，便于在需要处替换/注入 |
| `Config` / `LoadConfig` | 从环境变量读取配置，启动时校验 |
| `NewClientFromEnv` | 直接按环境变量构建客户端 |
| `OpenAPIError` | OpenAPI 调用失败，携带请求地址、错误码、trace_id、HTTP 状态 |
| `APIError` | 获取凭证接口的业务错误 |
| `OpenAPIErrorCode` | 公共错误码常量（约 70 个，含符号名） |
| `Payload` / `OpCode` | 官方网关数据结构 `{id,op,d,s,t}` 与全部 opcode |
| `Intent` | 事件订阅位掩码（11 类，含事件类型常量） |
| `Dispatcher` | 事件分发器，按类型注册、并发分发、panic 隔离 |
| `Transport` | 传输层接口，Webhook 与 WebSocket 两种实现 |
| `WebhookTransport` | HTTP 回调接入，Ed25519 验签、地址验证、ACK 回包 |
| `WebSocketTransport` | 网关长连接，Hello/Identify/Resume/心跳/重连 |
| `Signer` | Ed25519 签名与验签，回调地址验证应答 |
| `GetGateway` / `GetGatewayBot` | 获取 WSS 接入点（含分片建议与 session 限额） |
| `ShardID` | 分片计算：`(guild_id >> 22) % num_shards` |

## 安装

```bash
go get github.com/fouc3/qq-bot-sdk
```

该仓库为私有仓库，需配置：

```bash
go env -w GOPRIVATE=github.com/fouc3/*
```

## 默认值与覆盖

请求地址与请求头默认使用文档规定的值，**每一项都可以覆盖**：

| 项目 | 默认值 | 覆盖方式 |
| --- | --- | --- |
| 请求地址 | `https://api.bot.qq.com` | `WithBaseURL` / `Config.BaseURL` |
| 鉴权头 | `Authorization: QQBot {ACCESS_TOKEN}` | `WithHeader("Authorization", ...)` |
| 内容类型 | `Content-Type: application/json; charset=utf-8` | `WithHeader("Content-Type", ...)` |
| 任意其它请求头 | 无 | `WithHeader` / `WithHeaders` / `Config.Headers` |
| HTTP 客户端 | `&http.Client{Timeout: 15s}` | `WithHTTPClient` |

请求头名大小写不敏感（按 HTTP 规范）。**一旦覆盖了 `Authorization`，SDK 就不再自动获取凭证**，直接使用调用方给的值——这适用于走代理或自带 token 的场景。

```go
client := qqbotsdk.NewClient("APPID", "CLIENTSECRET",
	qqbotsdk.WithBaseURL("https://api.example.com"),
	qqbotsdk.WithHeader("Content-Type", "application/json"),
	qqbotsdk.WithHeaders(http.Header{"X-Custom": {"v"}}),
)
```

## 启动配置

凭证来自**二选一**的环境变量，在启动时校验，缺失则立即失败，而不是等到第一次 API 调用才暴露：

| 变量 | 说明 |
| --- | --- |
| `ACCESS_TOKEN` | 已签发的凭证。设置后直接使用，不再获取、不再刷新 |
| `APPID` + `CLIENTSECRET` | 机器人凭证。SDK 自动获取并刷新 access_token |

`ACCESS_TOKEN` 优先：它已设置时不会再使用 `APPID`/`CLIENTSECRET`，便于运维临时固定一个 token 而不必清空其它变量。两者都不完整时返回 `ErrNoCredentials`。

```go
import "errors"

client, err := qqbotsdk.NewClientFromEnv()
if errors.Is(err, qqbotsdk.ErrNoCredentials) {
	log.Fatal("请设置 ACCESS_TOKEN，或同时设置 APPID 与 CLIENTSECRET")
}
if err != nil {
	log.Fatal(err)
}
```

也可以自行组装配置：

```go
cfg, err := qqbotsdk.LoadConfig()
// 或 cfg := qqbotsdk.Config{AccessToken: "...", BaseURL: "..."}
client, err := qqbotsdk.NewClientFromConfig(cfg, qqbotsdk.WithHTTPClient(hc))
```

`Config.String()` 会把密钥与 token 脱敏为 `<set>`，可安全写日志。

## 错误处理

官方文档明确：**不要依据 `message` 判断成败**，它随时可能调整；请依据错误码判断。

平台有两种失败信号，SDK 都收进 `OpenAPIError`：

1. **HTTP 状态码**：401 认证失败、404 未找到 API、405 方法不允许、429 频率限制、500/504 处理失败；
2. **响应体 `err_code`**：可能伴随 HTTP 200 一起返回。

```go
var openAPIErr *qqbotsdk.OpenAPIError
if errors.As(err, &openAPIErr) {
	openAPIErr.Code      // err_code，未提供时为 0
	openAPIErr.StatusCode// HTTP 状态码
	openAPIErr.Method    // 请求方法
	openAPIErr.URL       // 完整请求地址
	openAPIErr.TraceID   // trace_id，取自响应体或 X-Tps-trace-ID 响应头
	openAPIErr.Body      // 截断后的原始响应体，便于排查

	switch {
	case openAPIErr.IsAuthFailure():   // 401 或 token/appid 类错误码
	case openAPIErr.IsNotFound():      // 404
	case openAPIErr.IsMethodNotAllowed():// 405
	case openAPIErr.IsRateLimited():   // 429 或限频类错误码
	case openAPIErr.IsServerError():   // 500 / 504
	case openAPIErr.Retryable():       // 文档标注"可重试"的系统错误
	}
}
```

注意 `201` / `202` 是**异步成功**，但文档说明它们仍会返回 error body，因此也会以 `OpenAPIError` 形式出现；用 `IsAsyncAccepted()` 识别，按"待审核"处理而非硬失败。

判断具体错误码：

```go
if qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrSafeHit) { ... }
if openAPIErr.HasCode(qqbotsdk.ErrTriggerChannelRateLimit) { ... }
```

> `HasCode` 没有命名为 `Is`，是为了避免与 `errors.Is` 的约定（接收 `error`）混淆。

错误码常量覆盖文档「公共错误码」一节，命名与文档符号名一致（如 `ErrCheckAdminFailed` = 11281），`OpenAPIErrorCode.String()` 可还原符号名。文档中 `ErrorWrongAppid` 一名对应 11251/11261/11275 三个码，SDK 分别命名为 `ErrWrongAppID`、`ErrMissingAppID`、`ErrNoAppID`。

## 凭证获取

官方文档：[获取访问凭证](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/access-token.html)

- 接口：`POST https://api.bot.qq.com/app/getAppAccessToken`
- 请求体：`{"appId": "...", "clientSecret": "..."}`
- 成功响应：`{"access_token": "...", "expires_in": 7200}`

`expires_in` 在参数表中标注为 number，官方示例中却返回字符串 `"7200"`，因此 SDK 对两种形式都做兼容解析。

该接口的业务失败通过响应体 `code` 返回，HTTP 仍为 200，因此以 `*APIError` 报出：

```go
var apiErr *qqbotsdk.APIError
if errors.As(err, &apiErr) {
	switch apiErr.Code {
	case qqbotsdk.ErrCodeInvalidCredential: // 100016：AppID 或 ClientSecret 错误
	case qqbotsdk.ErrCodeAppIDInvalid:      // 100007：AppID 无效或机器人状态异常
	case qqbotsdk.ErrCodeTooManyRequests:   // 100001：请求过于频繁
	case qqbotsdk.ErrCodeBotNotFound:       // 10004：机器人不存在
	}
}
```

### 缓存与刷新

`AccessToken` 会在本地缓存凭证，避免每次调用都请求接口：

- 凭证剩余有效期大于 **60 秒** 时直接复用缓存；
- 进入到期前 60 秒窗口后自动重新获取。该窗口与平台行为一致——官方说明在上一个凭证接近过期 60 秒内请求会签发新凭证；
- 并发调用共享同一次请求，不会产生重复的网络开销。

需要立即换新凭证时调用 `InvalidateToken()`。

## 事件订阅与通知

官方文档：[通用数据结构](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/event-emit/payload.html)、[Webhook 方式](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/event-emit/webhook.html)、[WebSocket 方式](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/event-emit/websocket.html)。

两种接入方式共用**同一套**上下行结构，SDK 据此定义 `Payload`：

```json
{ "id": "event_id", "op": 0, "d": {}, "s": 42, "t": "GATEWAY_EVENT_NAME" }
```

| 字段 | 说明 |
| --- | --- |
| `id` | 事件 id |
| `op` | opcode，见 `OpCode` |
| `s` | 下行序列号，心跳时需回传客户端收到的最新值 |
| `t` | 事件类型，`op` 为 `OpDispatch` 时有效 |
| `d` | 事件内容，格式随 `t` 变化 |

`d` 的结构随事件类型而变，因此用 `event.DecodeData(&v)` 显式解码，而不是让 SDK 猜测。

### opcode

| 值 | 名称 | 接入 | 行为 |
| --- | --- | --- | --- |
| 0 | Dispatch | 两者 | 收 |
| 1 | Heartbeat | ws | 收/发 |
| 2 | Identify | ws | 发 |
| 6 | Resume | ws | 发 |
| 7 | Reconnect | ws | 收 |
| 9 | Invalid Session | ws | 收 |
| 10 | Hello | ws | 收 |
| 11 | Heartbeat ACK | ws | 收/回 |
| 12 | HTTP Callback ACK | webhook | 回 |
| 13 | 回调地址验证 | webhook | 收 |

### intents

`Intent` 是位掩码，需要哪类事件就把对应位置 1。`IntentsFor` 做组合，`IntentForEvent` 可由事件类型反查所属类别：

```go
intents := qqbotsdk.IntentsFor(
	qqbotsdk.IntentPublicGuildMessages, // 1<<30，@机器人消息
	qqbotsdk.IntentGroupAndC2CEvent,    // 1<<25，群/单聊
)
```

基础类别（`GUILDS`、`GUILD_MEMBERS`、`PUBLIC_GUILD_MESSAGES`）默认有权限，其余需申请。**订阅无权限的 intents 会被网关报错并直接关闭连接**；若权限被取消，当前连接不报错但收不到事件，重连才报错。

### 注册与分发

`Dispatcher` 按事件类型路由，`WildcardEventType`（`"*"`）接收全部事件。处理器并发执行，**panic 与错误都被隔离**：一个处理器失败不影响其它处理器，也不会带崩进程。

```go
client, err := qqbotsdk.NewClientFromEnv() // 读环境变量
if err != nil {
	log.Fatal(err)
}

reg := client.Register(qqbotsdk.EventGroupAtMessageCreate, qqbotsdk.EventHandlerFunc(
	func(ctx context.Context, event *qqbotsdk.Event) error {
		var data struct {
			Content string `json:"content"`
		}
		if err := event.DecodeData(&data); err != nil {
			return err
		}
		return reply(ctx, data.Content)
	},
))
defer reg.Cancel() // 动态注销
```

`Event` 内嵌 `*Payload`，因此 `event.Op`、`event.Type`、`event.ID`、`event.Sequence()` 直接可用；另有 `event.Transport` 标明来源（`webhook` / `websocket`）与 `event.ReceivedAt`。

分发有两个入口：`Dispatch`（不等待，供传输层读取循环使用）与 `DispatchSync`（等待全部处理器，返回合并错误）。传输层与处理器失败统一经 `ErrorHandler` 上报，可自行替换：

```go
client := qqbotsdk.NewClient(appID, clientSecret,
	qqbotsdk.WithDispatcher(qqbotsdk.NewDispatcher(
		qqbotsdk.WithMaxConcurrency(32),
		qqbotsdk.WithErrorHandler(func(ctx context.Context, e *qqbotsdk.Event, err error) {
			slog.Error("handler failed", "err", err, "type", e.Type)
		}),
	)),
)
```

### Webhook

平台仅回调 **80 / 443 / 8080 / 8443** 端口，回调地址须为 HTTPS。SDK 会校验监听端口，不在其列直接报错，避免"静默收不到回调"。

签名算法是 **Ed25519**（不是 HMAC）。`Bot Secret` 经重复填充得到 32 字节 seed，派生密钥对；签名体为 **`timestamp + body`**，签名值以 hex 放在 `X-Signature-Ed25519`，时间戳在 `X-Signature-Timestamp`。SDK 按文档实现，并用文档给出的 seed、公钥、op13 签名三组向量做了断言。

**验签先于解析**：未通过验签的回调一律返回 401，不会进入任何处理器。

```go
webhook := qqbotsdk.NewWebhookTransport(
	qqbotsdk.WithWebhookAddr(":8080"),
	qqbotsdk.WithWebhookPath("/events"),
	qqbotsdk.WithWebhookSecret(botSecret),
	qqbotsdk.WithWebhookAppID(appID),
)
client.UseTransport(webhook)
```

已有 HTTP 服务时可只取 handler 挂载：

```go
handler, err := webhook.Handler(client.Dispatcher().Dispatch)
mux.Handle("/events", handler)
```

回调地址验证（op 13）由 SDK 自动应答；普通事件回 op 12 ACK 后异步处理。若希望"处理器失败则让平台重试"，开启 `WithWebhookSyncDispatch(true)`，此时处理器出错会返回 HTTP 500（由 `Client.Start` 自动接线）。

### WebSocket

地址取自 `GetGateway`（`GET /gateway`）或 `GetGatewayBot`（`GET /gateway/bot`，另含建议分片数与会话限额）。SDK 负责全生命周期：

- **Hello**（op 10）读取心跳周期；
- **Identify**（op 2）携带 `QQBot {AccessToken}`、intents、shard、properties；
- **心跳**（op 1）按周期发送，`d` 为收到的最新 `s`，首次为 `null`；收到 op 11 确认；
- **Resume**（op 6）断线重连时携带 `session_id` 与 `seq`，网关自动补发遗漏事件；
- **Reconnect**（op 7）与 **Invalid Session**（op 9）分别触发重连与重新 identify；
- **重连退避**，并提供 `tokenFunc` 在每次连接前重新取 token，避免 token 过期后无法恢复。

```go
gateway, err := client.GetGatewayBot(ctx)
socket := qqbotsdk.NewWebSocketTransport(gateway.URL,
	qqbotsdk.WithIntents(intents),
	qqbotsdk.WithShard(qqbotsdk.Shard{ID: 0, Count: 1}),
)
client.UseTransport(socket)
```

分片按频道 id 哈希：`ShardID(guildID, numShards)` 即文档的 `(guild_id >> 22) % num_shards`。无需分片用 `[0, 1]`。

WebSocket 关闭码（`CloseCode`）按文档分类：`CanResume()` / `CanIdentify()` / `Fatal()`。**致命关闭码（如 intent 无权限 4014、机器人被封禁 4915）会停止重连**——继续重连只会被再次拒绝；其余情况按退避重连。

### 生命周期

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := client.Start(ctx); err != nil { // 启动全部 transport
	log.Fatal(err)
}
defer client.Stop(context.Background()) // 反向停止并等待在途处理器
```

可只用 Webhook、只用 WebSocket，或两者同时启用。`Start` 期间任一 transport 启动失败，已启动的会被回滚停止；取消 `ctx` 等同于 `Stop`。

## 开发

```bash
go test -race ./...
go vet ./...
gofmt -l .
```

## 说明

- 本 SDK 为独立实现，与官方 `tencent-connect/botgo` 无关。
- 官方文档「安全和授权」页的**验签示例签名无法用同页给出的密钥与消息复现**，本 SDK 以可复现的 seed、公钥与 op13 向量为准，并在测试中注明了该差异。
