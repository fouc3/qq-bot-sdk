# qq-bot-sdk

QQ 机器人（QQ Bot）开放平台 SDK，Go 实现。参考官方文档：[QQ 机器人开发文档 · API v2](https://bot.q.qq.com/wiki/develop/api-v2/)。

模块路径：`github.com/fouc3/qq-bot-sdk`

## 已实现

| 能力 | 说明 |
| --- | --- |
| `Client` | OpenAPI 客户端，可配置 base URL 与 `http.Client` |
| `GetAppAccessToken` | 调用 `POST /app/getAppAccessToken`，强制获取新凭证 |
| `AccessToken` | 带缓存的凭证获取，到期前自动刷新 |
| `InvalidateToken` | 主动作废缓存凭证 |
| `TokenSource` | 凭证来源接口，便于在需要处替换/注入 |
| `APIError` | 业务错误码封装（HTTP 200 也会返回错误） |

## 安装

```bash
go get github.com/fouc3/qq-bot-sdk
```

该仓库为私有仓库，需配置：

```bash
go env -w GOPRIVATE=github.com/fouc3/*
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"

	qqbotsdk "github.com/fouc3/qq-bot-sdk"
)

func main() {
	client := qqbotsdk.NewClient("你的 AppID", "你的 ClientSecret")

	token, err := client.AccessToken(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(token.Value)
	fmt.Println(token.ExpiresAt)
	// 调用 OpenAPI 时的请求头：
	fmt.Println(token.AuthorizationHeader()) // QQBot ACCESS_TOKEN
}
```

## 凭证获取

官方文档：[获取访问凭证](https://bot.q.qq.com/wiki/develop/api-v2/dev-prepare/access-token.html)

- 接口：`POST https://api.bot.qq.com/app/getAppAccessToken`
- 请求体：`{"appId": "...", "clientSecret": "..."}`
- 成功响应：`{"access_token": "...", "expires_in": 7200}`

`expires_in` 在参数表中标注为 number，官方示例中却返回字符串 `"7200"`，因此 SDK 对两种形式都做兼容解析。

### 缓存与刷新

`AccessToken` 会在本地缓存凭证，避免每次调用都请求接口：

- 凭证剩余有效期大于 **60 秒** 时直接复用缓存；
- 进入到期前 60 秒窗口后自动重新获取。该窗口与平台行为一致——官方说明在上一个凭证接近过期 60 秒内请求会签发新凭证；
- 并发调用共享同一次请求，不会产生重复的网络开销。

需要立即换新凭证时调用 `InvalidateToken()`。

### 错误处理

业务失败时平台仍返回 HTTP 200，错误信息在响应体的 `code` 中，**不要**只依据 HTTP 状态码判断成败：

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

`message` 仅用于人工排查，内容可能随时调整，请勿据此判断错误类型。

## 开发

```bash
go test -race ./...
go vet ./...
gofmt -l .
```

## 说明

本 SDK 为独立实现，与官方 `tencent-connect/botgo` 无关。
