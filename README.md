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

## 开发

```bash
go test -race ./...
go vet ./...
gofmt -l .
```

## 说明

本 SDK 为独立实现，与官方 `tencent-connect/botgo` 无关。
