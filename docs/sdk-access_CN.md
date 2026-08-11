# sdk/access SDK 参考

`sdk/access` 包集中了代理的入站请求认证。它提供轻量级的管理器来串联凭据提供者，使服务器可以在 CLI 运行时内外复用相同的访问控制逻辑。

## 导入

```go
import (
    sdkaccess "github.com/shinmentakezo07/shinway/v7/sdk/access"
)
```

通过 `go get github.com/shinmentakezo07/shinway/v7/sdk/access` 添加模块。

## 提供者注册表

提供者被全局注册，然后以快照方式挂载到 `Manager`：

- `RegisterProvider(type, provider)` 安装一个预先初始化的提供者实例。
- 首次看到某个 `type` 时保留其注册顺序。
- `UnregisterProvider(type)` 按类型标识移除提供者。
- `SetExclusiveProvider(type)` 将 `RegisteredProviders()` 限制为单个键；`ClearExclusiveProvider()` 移除该限制。
- `RegisteredProviders()` 按注册顺序返回提供者。

```go
type myProvider struct{}
func (myProvider) Identifier() string { return "my-access" }
func (myProvider) Authenticate(ctx context.Context, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
    if r.Header.Get("X-Custom-Key") != "secret" {
        return nil, sdkaccess.NewInvalidCredentialError()
    }
    return &sdkaccess.Result{Provider: "my-access", Principal: "custom"}, nil
}

func init() {
    sdkaccess.RegisterProvider("my-access", myProvider{})
}
```

## 管理器生命周期

```go
manager := sdkaccess.NewManager()
manager.SetProviders(sdkaccess.RegisteredProviders())
```

- `NewManager` 构造空管理器。
- `SetProviders` 使用防御性拷贝替换提供者切片。
- `Providers` 获取可安全跨 goroutine 遍历的快照。

如果管理器本身为 `nil` 或未配置任何提供者，调用返回 `nil, nil`，调用方可将访问控制视为禁用。

## 认证请求

```go
result, authErr := manager.Authenticate(ctx, req)
switch {
case authErr == nil:
    // 访问已授权。result.Principal 与 result.Metadata 标识调用方。
case sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeNoCredentials):
    // 未提供凭据。
case sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeInvalidCredential):
    // 提供了凭据但无效。
default:
    // 提供者特定失败；传播或映射为 500。
}
```

管理器按顺序评估提供者，并在第一个成功后短路。返回 `AuthErrorCodeNotHandled` 的提供者会被跳过，以便继续链路。缺失凭据的结果会被记住，而无效凭据的结果在最后具有更高优先级。

## 错误辅助函数

- `NewNoCredentialsError()` – 未提供凭据。
- `NewInvalidCredentialError()` – 提供了凭据但被拒绝。
- `NewNotHandledError()` – 提供者不处理该请求形态。
- `NewInternalAuthError(message, cause)` – 意外失败。
- `IsAuthErrorCode(authErr, code)` – 类型化错误检查。

## 提供者接口

```go
type Provider interface {
    Identifier() string
    Authenticate(ctx context.Context, r *http.Request) (*Result, *AuthError)
}
```

`Result` 携带 `Provider`、`Principal` 与可选的 `Metadata`。返回 `(nil, nil)` 表示匿名访问。

## 与服务的集成

嵌入式服务在 `sdk/shinway` 的 builder 运行时自动接线访问管理器：它调用 `SetProviders(sdkaccess.RegisteredProviders())` 并配置访问管理的路由。可通过 `Builder.WithRequestAccessManager` 提供自定义管理器以定制链路。
