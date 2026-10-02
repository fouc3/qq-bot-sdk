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
| `Intent` | 事件订阅位掩码（12 类，含全部事件类型常量） |
| `Dispatcher` | 事件分发器，按类型注册、并发分发、panic 隔离 |
| `DecodeEvent` / `EventDataFor` | 按事件类型解码事件体，46 个事件全部有结构 |
| `Transport` | 传输层接口，Webhook 与 WebSocket 两种实现 |
| `WebhookTransport` | HTTP 回调接入，Ed25519 验签、地址验证、ACK 回包 |
| `WebSocketTransport` | 网关长连接，Hello/Identify/Resume/心跳/重连 |
| `Signer` | Ed25519 签名与验签，回调地址验证应答 |
| `GetGateway` / `GetGatewayBot` | 获取 WSS 接入点（含分片建议与 session 限额） |
| `ShardID` | 分片计算：`(guild_id >> 22) % num_shards` |
| `SendC2CMessage` | 发送单聊消息 |
| `SendC2CStreamMessage` | 流式发送单聊消息 |
| `SendGroupMessage` | 发送群聊消息 |
| `SendChannelMessage` | 发送子频道消息（JSON 或 multipart 带图） |
| `SendDirectMessage` | 发送频道私信 |
| `CreateDirectMessageSession` | 创建频道私信会话 |
| `RecallC2CMessage` / `RecallGroupMessage` | 撤回单聊／群聊消息 |
| `RecallChannelMessage` / `RecallDirectMessage` | 撤回子频道／私信消息 |
| `UploadC2CFile` / `UploadGroupFile` | 富媒体 URL 上传，或分片上传合并 |
| `PrepareC2CUpload` / `FinishC2CUploadPart` | 分片上传的预上传与分片完成（群聊同理） |
| `Message` / `Keyboard` / `MessageArk` / `MessageEmbed` | 消息类型与卡片、按钮等请求结构 |
| `AddReaction` / `RemoveReaction` | 频道消息的表情表态（添加／删除） |
| `ReactionUsers` | 拉取某条消息某表情的表态用户（分页） |
| `GetBotInfo` | 获取机器人自身详情 `GET /users/@me` |
| `GetJoinedGuilds` | 获取机器人已加入的频道列表（分页） |
| `GenerateShareLink` | 生成机器人分享链接 `POST /v2/generate_url_link` |
| `RespondInteraction` | 回应互动事件 `PUT /interactions/{interaction_id}` |
| `GetGroupInfo` / `GetGroupBotState` | 群基本信息、机器人群内状态 |
| `ListGroupJoinRequests` / `ApproveGroupJoinRequest` | 入群申请列表与审批 |
| `GetGroupRestrictChatSetting` / `SetGroupMemberMute` | 群禁言查询、成员禁言设置 |
| `ListJoinApprovalStrategies` / `CreateJoinApprovalStrategy` | 入群自动审批策略列表与创建 |
| `UpdateJoinApprovalStrategy` / `DeleteJoinApprovalStrategy` / `ExecuteJoinApprovalStrategy` | 策略修改、删除、执行 |
| `UpdateJoinApprovalStrategyWhitelist` | 策略白名单增删 |
| `ListGroupMembers` / `GetGroupMember` / `BatchRemoveGroupMembers` ⚠️ | 群成员列表、单个成员、批量移除（**需申请权限**，见群管理章节） |
| `ListGroupBlacklist` / `UpdateGroupBlacklist` ⚠️ | 群黑名单查询与增删（**需申请权限**，见群管理章节） |
| `GetMenu` / `SetMenu` | 自定义菜单（命令列表）查询与整体覆盖 `GET/PUT /v2/menu` |
| `ListPanels` / `CreatePanel` / `GetPanel` | 指令面板列表、创建、详情 |
| `UpdatePanel` / `DeletePanel` / `UpdatePanelTargets` | 指令面板修改、删除、关联对象增删 |
| `Menu` / `Panel` 校验 | 本地校验文档规定的类型、数量与 https 链接要求 |
| 消息错误码 | `errcode_message.go`，约 60 个按接口归类的错误码 |

## 代码结构

SDK 是**单包**（`package qqbotsdk`）。这是有意为之：Go 要求方法与其接收者类型同包，所以 51 个 `Client` 方法若按目录拆分，就只能变成多个 client 类型或自由函数——调用方式会变复杂。因此用**文件名前缀分层**，目录既分层又能聚拢排序：

| 前缀 / 文件 | 职责 |
| --- | --- |
| `client.go` `config.go` `auth.go` `errors.go` `doc.go` | 客户端、配置、凭证与令牌缓存、错误类型 |
| `api_*.go` | 每个端点域一个文件，装 `Client` 上的方法（gateway/message/file/reaction/bot/share/menu/panel） |
| `event.go` `event_dispatcher.go` `event_transport.go` | 网关数据结构与 intents、事件分发器、Transport 接口与生命周期 |
| `transport_webhook.go` `transport_websocket.go` `transport_sign.go` | 两种事件投递实现，以及回调签名 |
| `errcode*.go` | 错误码表：公共表 + 各端点族各一份 |

`go doc github.com/fouc3/qq-bot-sdk` 可看完整包文档（含分层导览）。

## 安装

```bash
go get github.com/fouc3/qq-bot-sdk
```

仓库是公开的，直接拉取即可；不需要 `GOPRIVATE` 之类的私有模块配置。

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

**配置由调用方传入**。SDK 不替你决定配置从哪来 —— 环境变量、命令行参数、配置文件、密钥管理服务都行，那是应用层的事。所以主路径是显式构造：

```go
client, err := qqbotsdk.NewClientFromConfig(qqbotsdk.Config{
	AppID:        "1234567890",
	ClientSecret: "******",
	// 或只给已签发的 token：
	// AccessToken: "******",
})
```

凭证**二选一**，在 `NewClientFromConfig` 里即时校验，缺失直接返回 `ErrNoCredentials`，不会拖到第一次 API 调用才暴露：

| 字段 | 说明 |
| --- | --- |
| `AccessToken` | 已签发的凭证。设置后直接使用，不再获取、不再刷新 |
| `AppID` + `ClientSecret` | 机器人凭证。SDK 自动获取并刷新 access_token |

最简形式可以直接给两个字符串：

```go
client := qqbotsdk.NewClient("1234567890", "******")
```

> `NewClient` 不返回 error，所以它**不做启动期校验**：凭证缺失时不会立刻失败，而是在**首次请求**返回同一个 `ErrNoCredentials`（实测确认）。要在启动时就发现问题就用 `NewClientFromConfig`。

需要定制时用 Option 叠加：

```go
client, err := qqbotsdk.NewClientFromConfig(cfg,
	qqbotsdk.WithHTTPClient(hc),
	qqbotsdk.WithBaseURL("https://api.bot.qq.com"),
)
```

`Config` 是纯数据结构（`Validate()` 可单独调用），`Config.String()` 会把密钥与 token 脱敏为 `<set>`，可安全写日志。

### 可选便利：从环境变量读取

如果不想自己写加载逻辑，SDK 提供一个便利函数，直接读 `os.LookupEnv`：

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

| 变量 | 说明 |
| --- | --- |
| `ACCESS_TOKEN` | 已签发的凭证。设置后直接使用，不再获取、不再刷新 |
| `APPID` + `CLIENTSECRET` | 机器人凭证。SDK 自动获取并刷新 access_token |

> **注意这个便利函数内含一条策略**：`ACCESS_TOKEN` 优先于 `APPID`/`CLIENTSECRET`（便于运维临时固定一个 token 而不必清空其它变量）。**优先级属于应用层决策** —— 如果你的程序想让 `--appid` 参数覆盖环境变量，就不要用 `NewClientFromEnv`，自己读环境变量组装 `Config` 再传给 `NewClientFromConfig`。

`LoadConfig()` 返回同样的 `Config`，便于"先读环境变量、再按自己的规则覆盖其中几项"：

```go
cfg, err := qqbotsdk.LoadConfig() // 读 env，规则同上
cfg.BaseURL = "https://staging.example.com"
client, err := qqbotsdk.NewClientFromConfig(cfg)
```

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
		var data qqbotsdk.GroupMessageCreateData
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

### 事件内容与解码

46 个事件类型**全部**有对应的 Go 结构。用 `DecodeEvent` 按事件类型自动解出，不必自己写匿名 struct：

```go
value, err := qqbotsdk.DecodeEvent(payload)
switch data := value.(type) {
case *qqbotsdk.GroupMessageCreateData:
	fmt.Println(data.Content, data.GroupOpenID, data.Author.MemberOpenID)
case *qqbotsdk.C2CMessageCreateData:
	fmt.Println(data.Content)
case *qqbotsdk.FriendAddData:
	fmt.Println(data.Scene, data.SceneParam) // 分享链接带来的 callback_data
}
```

`EventDataFor(事件类型)` 返回该事件应解入的空结构，未知类型返回 `nil`；`DecodeEvent` 对未知类型与不匹配的事件体都会**明确报错**，不会静默解成空值。

| 分组 | 事件 | 结构 |
| --- | --- | --- |
| 单聊/群聊 | `C2C_MESSAGE_CREATE`、`GROUP_AT_MESSAGE_CREATE`、`GROUP_MESSAGE_CREATE` | `C2CMessageCreateData`、`GroupMessageCreateData` |
| 好友与群 | `FRIEND_ADD/DEL`、`GROUP_ADD_ROBOT`、`GROUP_DEL_ROBOT`、`GROUP_MEMBER_ADD/REMOVE`、`GROUP_JOIN_REQUEST`、`C2C_MSG_RECEIVE/REJECT`、`GROUP_MSG_RECEIVE/REJECT`、`SUBSCRIBE_MESSAGE_STATUS` | 各自结构 |
| 频道 | `GUILD_CREATE/UPDATE/DELETE`、`CHANNEL_CREATE/UPDATE/DELETE` | `GuildInfo`、`ChannelInfo` |
| 频道消息 | `AT_MESSAGE_CREATE`、`MESSAGE_CREATE`、`DIRECT_MESSAGE_CREATE` | `GuildMessage` |
| 消息删除/审核/表态 | `MESSAGE_DELETE`、`PUBLIC_MESSAGE_DELETE`、`DIRECT_MESSAGE_DELETE`、`MESSAGE_AUDIT_PASS/REJECT`、`MESSAGE_REACTION_ADD/REMOVE` | `MessageDelete`、`MessageAudited`、`MessageReaction` |
| 论坛 | `FORUM_THREAD_*`、`FORUM_POST_*`、`FORUM_REPLY_*`、`FORUM_PUBLISH_AUDIT_RESULT` | `ForumThreadEvent`、`ForumPostEvent`、`ForumReplyEvent`、`ForumAuditResult` |
| 互动 | `INTERACTION_CREATE` | `InteractionCreateData` |
| 音频 | `AUDIO_START/FINISH/ON_MIC/OFF_MIC` | `AudioAction` |
| 频道成员 | `GUILD_MEMBER_ADD/UPDATE/REMOVE` | `MemberWithGuildID` |
| 连接生命周期 | `READY`、`RESUMED` | `ReadyData`、`ResumedData` |

> `GuildMessage` **不是** `Message`：后者是发送消息的请求体，前者是频道消息事件的内容对象，两者字段不同，故意分开命名。

几处便利方法：`MessageScene.MsgIdx()/RefMsgIdx()/AuthToken()`（文档要求用 `msg_idx` 去重）、`MessageAttachment.IsVoice()/IsImage()`、`InteractionCreateData.NeedsResponse()`、`Emoji.IsBuiltinEmoji()`、`RichTextValue.PlainText()`、`ReadyData.ShardInfo()`。

**两个结构是推断而来的**，代码注释里都写明了依据：`AudioAction`（四个 `AUDIO_*` 事件）与 `MemberWithGuildID`（三个 `GUILD_MEMBER_*` 事件）。依据是——它们是官方定义的对象，却**没有任何接口使用**，而那几类事件没有公布字段表。

文档本身有两处自相矛盾，SDK 均已兼容：

- **论坛事件**：字段表说 `title`/`content` 是 `string`，而所有示例都是富文本对象数组 → `RichTextValue` 两种都收。
- **ID 类型**：论坛示例里 `guild_id` 是数字 `47129941624960822`、`emoji_info.id` 也是数字，字段表却写 string → 两种都解，否则真实事件会整体解码失败。

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

## 发送消息

官方文档：[消息收发概述](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/overview.html)。

### 接口一览

| 场景 | 接口 | 方法 |
| --- | --- | --- |
| 单聊 | `SendC2CMessage` | `POST /v2/users/{user_openid}/messages` |
| 单聊流式 | `SendC2CStreamMessage` | `POST /v2/users/{user_openid}/stream_messages` |
| 群聊 | `SendGroupMessage` | `POST /v2/groups/{group_openid}/messages` |
| 子频道 | `SendChannelMessage` | `POST /channels/{channel_id}/messages` |
| 子频道（带图） | `SendChannelMessageMultipart` | 同上，`multipart/form-data` |
| 频道私信 | `SendDirectMessage` | `POST /dms/{guild_id}/messages` |
| 创建私信会话 | `CreateDirectMessageSession` | `POST /users/@me/dms` |
| 撤回单聊 | `RecallC2CMessage` | `DELETE /v2/users/{user_openid}/messages/{message_id}` |
| 撤回群聊 | `RecallGroupMessage` | `DELETE /v2/groups/{group_openid}/messages/{message_id}` |
| 撤回子频道 | `RecallChannelMessage` | `DELETE /channels/{channel_id}/messages/{message_id}?hidetip=` |
| 撤回私信 | `RecallDirectMessage` | `DELETE /dms/{guild_id}/messages/{message_id}?hidetip=` |

### 消息类型

`msg_type` 决定哪个字段生效（单聊/群聊接口）：

| 值 | 常量 | 内容字段 |
| --- | --- | --- |
| 0 | `MsgTypeText` | `content` |
| 2 | `MsgTypeMarkdown` | `markdown` |
| 6 | `MsgTypeInputNotify` | `input_notify`（"输入中"状态，仅单聊） |
| 7 | `MsgTypeMedia` | `media`（需先上传拿 `file_info`） |

```go
// 被动回复一条群消息
if _, err := client.SendGroupMessage(ctx, groupOpenID, &qqbotsdk.Message{
	Content: "收到",
	MsgID:   event.ID,  // 事件里的 d.id
	MsgSeq:  1,
}); err != nil {
	log.Fatal(err)
}
```

### 消息按钮（内嵌键盘）

官方文档第一句就是关键：**「在 markdown 消息的基础上，支持消息最底部挂载按钮」**。所以键盘必须挂在 **markdown 消息**上：

```go
if _, err := client.SendC2CMessage(ctx, openID, &qqbotsdk.Message{
	MsgType:  qqbotsdk.MsgTypeMarkdown,           // 必须
	Markdown: &qqbotsdk.MessageMarkdown{Content: "**签到**\n请在下方选择："},
	Keyboard: &qqbotsdk.Keyboard{Content: &qqbotsdk.KeyboardContent{
		Rows: []qqbotsdk.Row{{Buttons: []qqbotsdk.Button{{
			ID: "btn_1",
			RenderData: &qqbotsdk.RenderData{
				Label:        "签到",     // 必填
				VisitedLabel: "已签到",   // 必填
				Style:        qqbotsdk.KeyboardStyleBlue,
			},
			Action: &qqbotsdk.Action{
				Type:          qqbotsdk.ActionTypeCallback,
				Data:          "signin",
				Permission:    &qqbotsdk.Permission{Type: qqbotsdk.PermissionTypeEveryone},
				UnsupportTips: "请升级 QQ 客户端", // 必填
			},
		}}}},
	}},
	MsgID:  data.ID, // 被动回复
	MsgSeq: 2,
}); err != nil {
	log.Fatal(err)
}
```

> **这是本 SDK 唯一一处"防静默失败"的校验**：把键盘挂在**纯文本**消息上时，平台**接受请求并直接丢掉按钮，不返回任何错误**（生产实测：消息到达后就是纯文本，点也没有按钮）。因此 `Message.Validate()` 会拒绝这种组合，`SendC2CMessage` / `SendGroupMessage` 都会先校验再发请求。
>
> 按钮必填字段（`render_data.label`、`visited_label`、`action.permission`、`action.data`、`action.unsupport_tips`）同样会被本地拦下 —— 字段表标注为必填，缺了只会渲染成空白。

### 回应互动（按钮回调）

按钮被点击后平台推送 `INTERACTION_CREATE` 事件，**必须调 `RespondInteraction` 回应**，否则用户客户端一直 loading 到超时。

**关键：回应不是在这条通道上回的。** 事件走 WebSocket 或 Webhook，但回应是一次**独立的 HTTPS 调用** `PUT /interactions/{interaction_id}`。Webhook 的 op12 ACK 只是"我收到你推送了"，**不携带互动结果**，回了 ACK 之后仍然必须调这个接口。

```go
client.RegisterFunc(qqbotsdk.EventInteractionCreate, func(ctx context.Context, event *qqbotsdk.Event) error {
	value, err := event.Decode()
	if err != nil {
		return err
	}
	data := value.(*qqbotsdk.InteractionCreateData)

	// 只有消息按钮(11)和快捷菜单(12)需要回应，其他类型无需回应
	if !data.NeedsResponse() {
		return nil
	}

	// 业务处理……
	code := qqbotsdk.InteractionCodeSuccess // 0
	if err := doSomething(ctx, data); err != nil {
		code = qqbotsdk.InteractionCodeFailed // 1
	}
	return client.RespondInteraction(ctx, data.ID, code) // data.ID 来自事件的 d.id
})
```

| code | 含义 |
| --- | --- |
| 0 `InteractionCodeSuccess` | 成功（零值，也是文档默认值） |
| 1 `InteractionCodeFailed` | 操作失败 |
| 2 `InteractionCodeTooFrequent` | 操作频繁 |
| 3 `InteractionCodeDuplicate` | 重复操作 |
| 4 `InteractionCodeNoPermission` | 没有权限 |
| 5 `InteractionCodeAdminOnly` | 仅管理员操作 |

几条必须知道的规则：

- **同一个 `interaction_id` 只应回应一次**，超时后失效。因为这条规则，SDK **不会**自动回应 —— 回应码承载的是你的业务语义（成功／无权限／频繁），自动回应会把它冲掉。
- **"只能回应一次"是由客户端保证的，不是靠服务端报错**：`code=0` 回应后按钮进入终态、用户根本点不了第二次；而生产实测对同一 `interaction_id` 再调一次接口**依然返回成功**。所以别指望靠捕获错误发现自己重复回应了 —— 自己保证只回一次。
- `interaction_id` 取自事件的 **`d.id`**。文档特别提醒**不带 `INTERACTION_CREATE:` 前缀**；SDK 会容忍你误带前缀（自动剥掉）并做转义，但正确写法就是 `data.ID`。
- 本地只校验 `interaction_id` 非空、`code` 在 0–5 之间。**平台侧条件靠平台兜底**：窗口过期、token 与 appid 不匹配这些只有平台知道，会以 `630001–630008` 的 `OpenAPIError` 返回（如 `ErrInteractionAppIDMismatch` 表示 AppID 与 interaction_id 不匹配）。

#### 点过一次的按钮就点不动了

生产实测：**回调按钮被点击一次后，那条消息里的按钮变成不可点击状态**（显示 `render_data.visited_label`）。这与文档的 `action.click_limit`（已弃用，标注"默认不限"）表面矛盾 —— 所以按钮变灰**不是因为点击次数上限**，而是按钮进入了"已访问"状态。

对写机器人的实际影响：**一条消息里的按钮只点一次**。想让用户能再点，就**再发一条带新键盘的消息**（每个消息各自带自己的键盘），已签到类功能就是这个套路：

```go
// 每次处理完，回一条新消息带新按钮，而不是指望旧按钮还能点
client.SendC2CMessage(ctx, openID, &qqbotsdk.Message{
	MsgType:  qqbotsdk.MsgTypeMarkdown,
	Markdown: &qqbotsdk.MessageMarkdown{Content: "签到成功，明天再来"},
	Keyboard: newKeyboard(), // 新消息 = 可点击的新按钮
	MsgID:    data.ID,
	MsgSeq:   1,
})
```

> **实测（生产）：`code` 决定按钮之后还能不能点。**
>
> - `code=0`（成功）→ 用户看到成功提示，该按钮进入终态，**不可再点**（显示 `visited_label`）；
> - `code≠0`（如 `1` 操作失败）→ 按钮**保持可点**，用户可以重试。
>
> 所以"失败可重试"是有平台支持的：失败时回 `InteractionCodeFailed`，让用户再点一次即可，不必另发消息。生产实测中同一个按钮用 `code=1` 回应后又被成功点击了一次。

#### 按钮行为实测汇总

以下全部来自真实环境（单聊 + markdown 键盘 + 真人点击），不是从文档推断的：

| 行为 | 实测结果 |
| --- | --- |
| `code=0` 成功回应 | 客户端提示**成功**；该按钮进入终态、**不可再点** |
| `code=1` 失败回应 | 客户端提示**操作失败**；按钮**保持可点** —— 同一按钮被连点两次，两次事件都到达 |
| 无 `group_id` 的相邻按钮 | **彼此独立**：一个被点过之后，旁边的照样能点（A3 在 A1 之后仍被点到） |
| 同一 `group_id` 的按钮 | **确认生效**：点掉一个后，同组另一个变灰、点不动（B2 点不了） |
| 指令按钮 `action.type=2` | **不产生 `INTERACTION_CREATE`**；把 `data` 填入输入框（实测填入 `/hello`），由用户自行发送 |
| 不同消息的键盘 | 互不影响；想让用户再点，就发一条带新键盘的新消息 |

> `group_id` 仅在 `action.type=1`（回调按钮）下有效，这一点文档有写，实测一致。

### 主动消息与被动消息

填了 `msg_id` 或 `event_id` 即被动回复，不填即主动消息。时效与次数（官方）：

| 场景 | 被动有效期 | 每条可回复次数 |
| --- | --- | --- |
| 单聊 | 60 分钟 | 4 次 |
| 群聊 | 5 分钟 | 5 次 |
| 频道 / 私信 | 5 分钟 | - |

同一 `msg_id` 会对相同 `msg_seq` 去重，重复发送同组合会失败 —— 多次回复同一消息请递增 `msg_seq`。主动消息受频控约束（如单聊未认证 5/qps 且 30/qpm），且用户可在客户端关闭接收。

### 流式消息（仅单聊）

首片不带 `stream_msg_id`，用响应 `id` 作为后续分片的 `stream_msg_id`，`index` 从 0 递增，末片 `input_state=10`：

```go
first, err := client.SendC2CStreamMessage(ctx, openID, &qqbotsdk.StreamMessage{
	InputMode:   qqbotsdk.StreamInputReplace,
	InputState:  qqbotsdk.StreamInputGenerating,
	Index:       0,
	ContentType: qqbotsdk.StreamContentMarkdown,
	ContentRaw:  "正在生成…",
	MsgID:       msgID,
	MsgSeq:      1,
})
// 后续分片携带 first.ID
```

`input_mode=replace` 表示 `content_raw` 是当前全量正文，且必须以已下发前缀开头，否则报 `ErrStreamPrefixImmutable`（40007）。

### 频道消息

`SendChannelMessage` 用 JSON，`content` / `embed` / `ark` / `image` / `markdown` **至少填一个**。需要同请求上传图片时用 `SendChannelMessageMultipart`，SDK 会按文档要求把对象/数组字段序列化为 JSON 字符串：

```go
resp, err := client.SendChannelMessageMultipart(ctx, channelID,
	&qqbotsdk.ChannelMessage{Content: "hi", Ark: ark},
	&qqbotsdk.ChannelMessageFile{FileName: "pic.png", Content: file},
)
```

> 频道发消息要求机器人保持 WebSocket 在线。

### 富媒体上传

图片/视频/语音/文件需先上传拿 `file_info`（有时效 `ttl`），再以 `msg_type=7` 发送。单聊与群聊的上传接口**互不通用**。

```go
// URL 上传
up, err := client.UploadC2CFile(ctx, openID, &qqbotsdk.FileUploadRequest{
	FileType: qqbotsdk.FileTypeImage,
	URL:      "https://example.com/a.png",
})
// 用 up.FileInfo 发消息
```

大文件走分片（推荐）：`PrepareC2CUpload` 拿 `upload_id` 与各分片预签名 URL → 逐片 HTTP PUT → 每片 `FinishC2CUploadPart` → 最后带 `upload_id` 调 `UploadC2CFile` 合并。群聊把 `C2C` 换成 `Group`。

`file_type`：1=图片(png/jpg)、2=视频(mp4)、3=语音(silk)、4=文件；超过软限制会降级为文件，超过硬限制(200MB)报错。

### 消息错误码

各接口的错误码已定义为常量（见 `errcode_message.go`），`OpenAPIErrorCode.String()` 会还原文档描述：

```go
if qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrReplyMsgIDExpired) {
	// 40034005：回复消息 msg_id 已过期，需尽快回复
}
```

常见：`ErrMsgTypeMismatch`(22006)、`ErrMessageContentViolation`(40034006)、`ErrMessageDeduplicated`(40054005)、`ErrActiveMessageRateLimited`(40034100)、`ErrFileTooLarge`(850031)、`ErrRecallTimeExceeded`(40064004)。

### 表情表态

官方文档：[表情表态](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/trans/emoji.html)。仅频道可用。

| 接口 | 路径 |
| --- | --- |
| `AddReaction` | `PUT /channels/{channel_id}/messages/{message_id}/reactions/{type}/{id}` |
| `RemoveReaction` | `DELETE 同上`（删除自己的表态） |
| `ReactionUsers` | `GET 同上?cookie=&limit=`（分页，limit 默认 20、最大 50） |

`type` 用 `EmojiTypeSystem`(1，系统表情，id 为数字) 或 `EmojiTypeEmoji`(2，Unicode emoji，id 为 emoji 本身)。`ReactionEmoji(id)` 可按 id 形态自动判定类型：

```go
emojiType, id, ok := qqbotsdk.ReactionEmoji("203") // 1, "203", true
if err := client.AddReaction(ctx, channelID, messageID, emojiType, id); err != nil {
	log.Fatal(err)
}
```

### 文本交互（内嵌格式）

[文本交互](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/message/trans/text-chain.html) 是**内容语法**，不是请求字段 —— 直接写在 `content` 或 `markdown` 里即可，无需额外结构：

| 能力 | 格式 | 可用场景 |
| --- | --- | --- |
| @某人 | `<qqbot-at-user id="" />` | 群聊、文字子频道 |
| @全体成员 | `<qqbot-at-everyone />` | 仅文字子频道（需权限） |
| 回车指令 | `<qqbot-cmd-enter text="xxx" />` | markdown；`text` 需 urlencode，≤100 字符 |

## 机器人信息

官方文档：[获取机器人详情](https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/users_me.get.html)、[获取机器人频道列表](https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/users_me_guilds.get.html)。

```go
info, err := client.GetBotInfo(ctx)     // GET /users/@me
guilds, err := client.GetJoinedGuilds(ctx, "", "", 20) // GET /users/@me/guilds
```

`BotInfo` 字段：`ID`、`Username`、`Avatar`、`Bot`，以及 `UnionOpenID` / `UnionUserAccount`（**需特殊申请并配置后才会返回**）与 `ShareURL` / `WelcomeMsg`。

> `ShareURL` 与 `WelcomeMsg` 只出现在官方**响应示例**里、未列入字段表，因此可能不返回；SDK 按可选字段处理。

`GetJoinedGuilds(ctx, after, before, limit)` 的游标与分页规则按文档：

- `before` 设置时先反序再分页；`before` 与 `after` 同时设置时 **`after` 无效**（SDK 两者都发，语义由平台决定）；
- `limit` 默认 100、**最大 100**，SDK 对超过 100 的值直接截断；
- 传空字符串即不发送该参数。

`GuildInfo` 字段：`ID`、`Name`、`Icon`、`OwnerID`、`Owner`、`JoinedAt`、`MemberCount`、`MaxMembers`、`Description`。

> 该接口的响应形状在官方文档里**自相矛盾**：字段表写 `{"guilds":[...]}`，而响应示例是裸数组 `[...]`。SDK **两种都能解析**，并已用两条测试分别覆盖。

### 分享链接

官方文档：[生成分享链接](https://bot.q.qq.com/wiki/develop/api-v2/autogen/api/v2_generate_url_link.post.html)。用于邀请用户添加机器人为好友。

```go
url, err := client.GenerateShareLink(ctx, "custom_data_123") // POST /v2/generate_url_link
```

`callback_data` 选填，**最长 32 字符**（按字符计，非字节），超长会在本地直接报错而不发请求。返回值即响应体的 `data.url`。

> **错误码冲突**：该页的 10001 是「请求参数异常」、10003 是「查询机器人信息异常」，而公共错误码表里 10001=UnknownAccount、10003=UnknownChannel。**同一数字在不同接口含义不同**，所以 `String()` 仍以公共表为准，处理该接口时请按本接口的语义解读这两个码。本页其余码（10002/10044/11004）已定义为 `ErrRequestHeaderInvalid`、`ErrUinFromHeaderFailed`、`ErrGenerateShareARKFailed`。

## 群管理

官方文档：[群管理](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/group/manage/)。共 17 个接口，分四组。

### 群信息与状态

```go
info, err := client.GetGroupInfo(ctx, groupOpenID)
// GroupOpenID / GroupName / GroupFingerMemo / GroupClassText / GroupTags / GroupMemberNum

state, err := client.GetGroupBotState(ctx, groupOpenID)
// MemberOpenID / JoinedAt / AllowProactiveMsg / RecvMsgSetting / MemberRole
```

`RecvMsgSetting` 用 `GroupRecvMsgAll`、`GroupRecvMsgOnlyMention`、`GroupRecvMsgMentionAndContext` 判断；`MemberRole` 用 `GroupRoleMember`/`GroupRoleOwner`/`GroupRoleAdmin`。`AllowProactiveMsg` 就是"群是否允许主动推送"，发主动消息前可以先查它。

### 入群申请与审批

```go
page, err := client.ListGroupJoinRequests(ctx, groupOpenID, "", 20) // limit 上限 50
for _, req := range page.List {
	log.Printf("%s 申请入群，风险提示 %q，来源 %s", req.Username, req.RiskTips, req.ApplySource)
}

err = client.ApproveGroupJoinRequest(ctx, groupOpenID, memberOpenID, &qqbotsdk.JoinRequestApproval{
	Op:                   qqbotsdk.JoinApprovalApprove, // 或 JoinApprovalDecline
	JoinRequestID:        request.JoinRequestID,
	RejectReason:         "人数已满",                     // op=decline 时可填
	AddToMemberBlacklist: true,                        // op=decline 时可填
})
```

**机器人必须是群管理员**才能审批，否则平台会拒绝。`JoinRequest` 与事件 `GROUP_JOIN_REQUEST` 的字段基本一致，`VerifyInfo`/`ReviewQA` 两个类型是**共用的**。

### 禁言

```go
setting, err := client.GetGroupRestrictChatSetting(ctx, groupOpenID)
// setting.GlobalRule.Mode = GroupMuteNone/Always/Schedule
// setting.GlobalRule.ScheduleRules / RecurringRules 是定时与周期规则
// setting.Members 是当前处于禁言中的成员

err = client.SetGroupMemberMute(ctx, groupOpenID, &qqbotsdk.SetGroupMemberMuteRequest{
	Members: []qqbotsdk.SetMemberMuteState{
		{Op: qqbotsdk.MemberMuteAdd, MemberOpenID: "M1", MuteExpireAt: "2026-07-22T10:00:00+08:00"},
		{Op: qqbotsdk.MemberMuteDelete, MemberOpenID: "M2"}, // del 时到期时间可留空表示立即解除
	},
})
```

同样需要管理员身份，**单次最多 20 个**、最长 30 天，且**只能禁言普通成员**（群主、管理员、机器人都不行）。

### 成员与黑名单

```go
members, err := client.ListGroupMembers(ctx, groupOpenID, "") // 每页最多 30 条，用 NextCursor 翻页
member, err := client.GetGroupMember(ctx, groupOpenID, memberOpenID)

result, err := client.BatchRemoveGroupMembers(ctx, groupOpenID, &qqbotsdk.BatchRemoveMembersRequest{
	MemberOpenIDs:        []string{"M1", "M2"}, // 单次最多 20 个
	AddToMemberBlacklist: true,                 // 同时拉黑，失败的在 result.BlacklistFailedOpenIDs 里
})

blacklist, err := client.ListGroupBlacklist(ctx, groupOpenID, "", 20) // limit 上限 100
result2, err := client.UpdateGroupBlacklist(ctx, groupOpenID, &qqbotsdk.UpdateBlacklistRequest{
	Op:            qqbotsdk.ListOpAdd, // 或 ListOpDelete
	MemberOpenIDs: []string{"M1"},
})
```

> **黑名单的一个坑**：文档明确写「目标成员在群中时无法加入黑名单」，所以流程是**先移除再拉黑**（或一步用 `BatchRemoveGroupMembers` 的 `AddToMemberBlacklist`）。加入失败的 openid 会在响应的 `fail_openids` 里返回。

### 入群自动审批策略

```go
list, err := client.ListJoinApprovalStrategies(ctx, "", 20) // limit 上限 50

created, err := client.CreateJoinApprovalStrategy(ctx, &qqbotsdk.CreateJoinApprovalStrategyRequest{
	GroupOpenIDs: []string{groupOpenID},   // 与 GroupIDs 二选一，最多 100 个
	IsEnable:     qqbotsdk.StrategyEnabled,
	Remark:       "自动放行",
})

updated, err := client.UpdateJoinApprovalStrategy(ctx, created.StrategyID,
	&qqbotsdk.UpdateJoinApprovalStrategyRequest{IsEnable: qqbotsdk.StrategyDisabled})

err = client.DeleteJoinApprovalStrategy(ctx, created.StrategyID)
err = client.ExecuteJoinApprovalStrategy(ctx, created.StrategyID)

whitelist, err := client.UpdateJoinApprovalStrategyWhitelist(ctx, created.StrategyID,
	&qqbotsdk.UpdateWhitelistRequest{
		Op:             qqbotsdk.ListOpAdd,
		WhitelistUsers: []string{"10001", "10002"}, // 号码用**字符串**，单次最多 10000 个
	})
```

- **群标识二选一且不能都传**：`group_openids` 与 `group_ids`（QQ 群号）互斥，同时传或都不传平台都会报错 —— SDK 在发请求前就拦下。
- `UpdateJoinApprovalStrategy` 的 `GroupAction` **群标识形式要与创建时一致**（文档原话），否则会失败。
- 白名单号码是 10000 个上限、且**故意用字符串**（文档说明是为了避开 JS 精度问题），SDK 直接按 `[]string` 收。

> **文档自相矛盾（已按示例实现）**：本组几个 GET 接口（`join_request_list`、`members`、`member_blacklist`、`join_approval_strategy`）把 `cursor`/`limit` 写在**「请求体」**下，但 `members` 的请求示例是 `GET .../members?cursor=` —— 用的是**查询参数**。SDK 按示例用查询参数发送，与面板列表接口的做法一致。

### 需要额外申请权限的接口（2026-10-02 实测）

以下 5 个接口的调用被平台拒绝，返回 `40012010`（`ErrGroupNoAPIPermission`，`应用无接口访问权限`）。**发现日期：2026-10-02。**

| 接口 | 路径 | 发现日期 | 返回 |
| --- | --- | --- | --- |
| `ListGroupMembers` | `GET /v2/groups/{group_openid}/members` | 2026-10-02 | `40012010` |
| `GetGroupMember` | `GET /v2/groups/{group_openid}/members/{member_openid}` | 2026-10-02 | `40012010` |
| `BatchRemoveGroupMembers` | `POST /v2/groups/{group_openid}/batch_remove_members` | 2026-10-02 | `40012010` |
| `ListGroupBlacklist` | `GET /v2/groups/{group_openid}/member_blacklist` | 2026-10-02 | `40012010` |
| `UpdateGroupBlacklist` | `POST /v2/groups/{group_openid}/member_blacklist` | 2026-10-02 | `40012010` |

**为什么确定是权限问题，而不是用错了**：同一时间、同一个群、同一个机器人做的对照实验 ——

- ✅ `SetGroupMemberMute`（禁言成员，改的是可逆状态）**成功**；
- ❌ 上表 5 个**全部被拒**；
- 机器人当时**已经是该群管理员**（`GetGroupBotState` 返回 `member_role=admin`）；
- 控制台里**可见的开关都已打开**（由账号所有者确认）。

三项对照都指向同一个结论：这几个接口要的是**应用级权限**，不是机器人身份或调用姿势。错误文案本身也写着"应用无接口访问权限"。

> 推测（非平台说明）：平台把「可逆的状态修改」与「不可逆 / 涉及成员隐私的操作」分开授权，所以禁言放开了、踢人和成员列表没有。

**在权限开通前的使用建议**：

```go
members, err := client.ListGroupMembers(ctx, groupOpenID, "")
if qqbotsdk.IsOpenAPIError(err, qqbotsdk.ErrGroupNoAPIPermission) {
	// 未开通：不要重试，也不要当成参数错误
	log.Println("该应用没有群成员管理权限，跳过成员列表")
	return
}
```

**一个影响可用性的副作用**：成员列表拿不到，意味着**成员的 `member_openid` 无法通过接口获取**。它只能从**群消息事件**里读：

```go
data := value.(*qqbotsdk.GroupMessageCreateData)
memberOpenID := data.Author.MemberOpenID // 这是拿到成员 openid 的唯一途径
```

所以禁言这类"按 openid 操作"的接口，必须**先等目标成员在群里发言**才能拿到它的 openid。生产测试就是这么做的（见 `production_group_member_test.go`）。

## 自定义菜单与指令面板

官方文档：[自定义菜单与指令面板](https://bot.q.qq.com/wiki/develop/api-v2/server-inter/menu-panel/)。

### 自定义菜单（命令列表）

展示在**单聊窗口底部**，设置后对所有用户生效，不支持按用户区分。

```go
// 查询：未配置时要同时处理两种情况 —— Menu 为 nil，或返回 items 为空的菜单对象
// （实测生产环境返回的是后者：version 有值、items 为空数组）
config, err := client.GetMenu(ctx)

// 修改：整体覆盖，返回新版本号
version, err := client.SetMenu(ctx, &qqbotsdk.Menu{Items: []qqbotsdk.MenuItem{
	{Type: qqbotsdk.MenuTypeSendMessage, Name: "帮助", SendMessage: "/help"},
	{Type: qqbotsdk.MenuTypeLink, Name: "官网", Link: "https://example.com"},
	{Type: qqbotsdk.MenuTypeMenu, Name: "更多", SubMenuItems: []qqbotsdk.SubMenuItem{
		{Type: qqbotsdk.SubMenuTypeSendMessage, Name: "设置", SendMessage: "/settings"},
	}},
	{Type: qqbotsdk.MenuTypeSwitch, Name: "搜索", Switch: &qqbotsdk.MenuSwitch{SwitchID: "search", Default: true}},
}})
```

按钮类型：`switch` 开关、`send_message` 填充输入框、`link` 跳转、`menu` 折叠项。**折叠项内不能再嵌套**，`switch` 只在一级有效。

`switch_id` 的用途：用户切换开关后平台会发一条消息，`ext` 里带上该标识（如 `search=1`），关闭则不带。

### 指令面板

支持 `c2c`（单聊）、`group`（群聊）、`channel`（文字子频道）、`dm`（频道私信）四种场景。**`channel` 与 `dm` 只能全局配置**，仅 `c2c`/`group` 支持按指定对象生效。

```go
panelID, err := client.CreatePanel(ctx, &qqbotsdk.PanelCreateRequest{
	Scope:      qqbotsdk.PanelScopeGroup,
	TargetType: qqbotsdk.PanelTargetSpecific,
	GroupOpenIDs: []string{"openid_group_001"},
	Panel: &qqbotsdk.Panel{
		Items: []qqbotsdk.PanelItem{
			{Type: qqbotsdk.PanelItemCommand, Name: "群签到", Desc: "每日签到"},
			{Type: qqbotsdk.PanelItemLink, Name: "更多服务", Link: "https://example.com", OnlyAdmin: true},
		},
		Remark: "群面板", // 最多 255 字符，不展示给用户
	},
})

page, err := client.ListPanels(ctx, qqbotsdk.PanelScopeGroup, "", 20) // limit 默认 20，上限 50
err = client.UpdatePanelTargets(ctx, panelID, &qqbotsdk.PanelTargetRequest{
	Op:           qqbotsdk.PanelTargetOpAdd,
	GroupOpenIDs: []string{"openid_group_003"},
})
```

`ListPanels` 必须传 `scope`；翻页用上页的 `NextCursor`，`IsEnd` 为 true 表示到底。`UpdatePanel` 只改元素与备注、**不动已关联对象**；关联对象用 `UpdatePanelTargets`（全局面板调用它会报 `ErrGlobalPanelNoTarget`）。

### 本地校验

`Menu.Validate()`、`Panel.Validate()`、`PanelCreateRequest.Validate()`、`PanelTargetRequest.Validate()`、`ValidateScope()` 会按文档约束提前报错，错误信息指明是哪一项：

- 菜单项 ≤10、子菜单项 ≤5；面板元素 ≤20；一次关联对象 ≤20；备注 ≤255 字符
- 类型枚举（按钮/元素/作用范围/操作类型）
- `link` 类型**必须以 `https://` 开头**
- `channel`/`dm` 场景传 `specific` 直接拒绝；`specific` 必须提供对应的 openid 列表

这些校验发生在**发请求之前**，因此非法输入不会产生一次注定失败的调用。

## 开发

```bash
go test -race ./...
go vet ./...
gofmt -l .
```

### 生产测试

`production_test.go`、`production_group_test.go`、`production_message_test.go`、`production_writes_test.go` 是**对真实平台**的集成测试，位于 `production` 构建标签之后 —— 普通 `go test` 绝不会联网或发出真实消息。凭证全部从环境变量读取，仓库里没有任何标识与密钥。

```bash
export QQBOT_APPID=... QQBOT_SECRET=...        # 机器人凭证
export QQBOT_BOT_QQ=...                        # 机器人的 QQ 号（OneBot 侧收件人）
export QQBOT_TEST_GROUP=...                    # 群聊测试用的群号（可选）
export ONEBOT_URL=http://localhost:3000        # OneBot 11 HTTP 客户端
export ONEBOT_KEY=... ONEBOT_SELF_QQ=...       # OneBot 凭证与其控制的 QQ
go test -tags production -run TestProduction -v .
```

覆盖内容：凭证与机器人身份、网关、WebSocket 上线（READY）、单聊与群聊的收发往返、markdown／按钮／富媒体上传／流式消息、面板全生命周期（创建→读取→更新→列表→删除）与关联对象增删、菜单读取／修改／还原、分享链接、以及只读接口。**送达**不是靠"平台没报错"，而是通过 OneBot 读取对方会话历史独立核对。

两个只有真实环境才会暴露的行为：

- 群里开启「接收所有消息」后触发的是 `GROUP_MESSAGE_CREATE`，其 `content` **仍带 `<@openid>` 提及标记**，并未按文档去除前缀。
- 流式消息能送达用户，但 QQ 客户端渲染为「暂不支持该消息类型」，因此送达核对只能认这个占位提示，比对正文必然失败。

## 说明

- 本 SDK 为独立实现，与官方 `tencent-connect/botgo` 无关。
- 官方文档「安全和授权」页的**验签示例签名无法用同页给出的密钥与消息复现**，本 SDK 以可复现的 seed、公钥与 op13 向量为准，并在测试中注明了该差异。
