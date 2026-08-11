# SDK 进阶：执行器与翻译器

本指南介绍如何使用 SDK 扩展嵌入式代理，支持自定义提供者与协议。你将：
- 实现一个与上游 API 通信的提供者执行器
- 为协议转换注册请求/响应翻译器
- 注册模型，使其出现在 `/v1/models`

示例使用 Go 1.26+ 与 v7 模块路径。

## 概念

- 提供者执行器：实现 `auth.ProviderExecutor` 的运行时组件，为给定提供者键（如 `gemini`、`claude`、`codex`）执行出站调用。执行器还可以实现 `RequestPreparer` 以在原始 HTTP 请求上注入凭据。
- 翻译器注册表：由 `sdk/translator` 路由的协议转换函数。内置处理器支持 OpenAI/Gemini/Claude/Codex 格式之间的转换；你可以注册新的。
- 模型注册表：按客户端/提供者发布可用模型列表，为 `/v1/models` 与路由提示提供支持。

## 1) 实现提供者执行器

创建满足 `auth.ProviderExecutor` 的类型。

```go
package myprov

import (
  "context"
  "net/http"

  coreauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
  clipexec "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

type Executor struct{}

func (Executor) Identifier() string { return "myprov" }

// 可选：向出站 HTTP 请求注入凭据
func (Executor) PrepareRequest(req *http.Request, a *coreauth.Auth) error {
  // 示例：req.Header.Set("Authorization", "Bearer "+a.Attributes["api_key"])
  return nil
}

func (Executor) Execute(ctx context.Context, a *coreauth.Auth, req clipexec.Request, opts clipexec.Options) (clipexec.Response, error) {
  // 基于 req.Payload（已翻译为提供者格式）构建 HTTP 请求
  // 使用按 auth 的传输层：transport := a.RoundTripper // 经 RoundTripperProvider
  // 执行调用并返回提供者 JSON 负载
  return clipexec.Response{Payload: []byte(`{"ok":true}`)}, nil
}

func (Executor) ExecuteStream(ctx context.Context, a *coreauth.Auth, req clipexec.Request, opts clipexec.Options) (*clipexec.StreamResult, error) {
  ch := make(chan clipexec.StreamChunk, 1)
  go func() { defer close(ch); ch <- clipexec.StreamChunk{Payload: []byte("data: {\"done\":true}\n\n")} }()
  return &clipexec.StreamResult{Chunks: ch}, nil
}

func (Executor) CountTokens(ctx context.Context, a *coreauth.Auth, req clipexec.Request, opts clipexec.Options) (clipexec.Response, error) {
  return clipexec.Response{Payload: []byte(`{"count":1}`)}, nil
}

func (Executor) HttpRequest(ctx context.Context, a *coreauth.Auth, req *http.Request) (*http.Response, error) {
  // 可选：执行预先构建的原始请求；必须关闭响应体。
  return nil, nil
}

func (Executor) Refresh(ctx context.Context, a *coreauth.Auth) (*coreauth.Auth, error) {
  // 可选：刷新令牌并返回更新的 auth
  return a, nil
}
```

在启动服务前向核心管理器注册执行器：

```go
core := coreauth.NewManager(sdkAuth.GetTokenStore(), nil, nil)
core.RegisterExecutor(myprov.Executor{})
svc, _ := shinway.NewBuilder().WithConfig(cfg).WithConfigPath(cfgPath).WithCoreAuthManager(core).Build()
```

如果你的认证条目使用 `"myprov"` 提供者，管理器会将请求路由到你的执行器。

## 2) 注册翻译器

处理器接受 OpenAI/Gemini/Claude/Codex 输入。要支持新的提供者格式，请在 `sdk/translator` 的默认注册表中注册翻译函数。

方向很重要：
- 请求：从入站协议注册到提供者协议
- 响应：从提供者协议注册回入站协议

示例：在 OpenAI Chat ↔ MyProv Chat 之间转换。

```go
package myprov

import (
  "context"
  sdktr "github.com/shinmentakezo07/shinway/v7/sdk/translator"
)

const (
  FOpenAI = sdktr.Format("openai.chat")
  FMyProv = sdktr.Format("myprov.chat")
)

func init() {
  sdktr.Register(FOpenAI, FMyProv,
    // 请求转换（model, rawJSON, stream）
    func(model string, raw []byte, stream bool) []byte { return convertOpenAIToMyProv(model, raw, stream) },
    // 响应转换（流式与非流式）
    sdktr.ResponseTransform{
      Stream: func(ctx context.Context, model string, originalReq, translatedReq, raw []byte, param *any) [][]byte {
        return convertStreamMyProvToOpenAI(model, originalReq, translatedReq, raw)
      },
      NonStream: func(ctx context.Context, model string, originalReq, translatedReq, raw []byte, param *any) []byte {
        return convertMyProvToOpenAI(model, originalReq, translatedReq, raw)
      },
    },
  )
}
```

当 OpenAI 处理器收到应路由到 `myprov` 的请求时，管道会自动使用已注册的转换函数。

## 3) 注册模型

通过向全局模型注册表注册模型（使用认证 ID/客户端 ID 与提供者名称），让模型出现在 `/v1/models` 下。

```go
models := []*registry.ModelInfo{
  { ID: "myprov-pro-1", Object: "model", Type: "myprov", DisplayName: "MyProv Pro 1" },
}
registry.GetGlobalRegistry().RegisterClient(authID, "myprov", models)
```

嵌入式服务器会自动为内置提供者执行此操作；对于自定义提供者，请在启动时（如加载认证后）或通过认证注册钩子注册。

## 凭据与传输层

- 使用 `Manager.SetRoundTripperProvider` 注入按 auth 的 `*http.Transport`（如代理）：
  ```go
  core.SetRoundTripperProvider(myProvider) // 按 auth 返回传输层
  ```
- 对于原始 HTTP 流程，实现 `PrepareRequest` 并/或调用 `Manager.InjectCredentials(req, authID)` 设置请求头。

## 测试提示

- 开启请求日志：管理 API GET/PUT `/v0/management/request-log`
- 切换调试日志：管理 API GET/PUT `/v0/management/debug`
- `config.yaml` 与认证存储的变更会被 watcher 自动热重载
