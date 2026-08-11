# Shinway SDK 指南

`sdk/shinway` 模块将代理以可复用的 Go 库形式暴露出来，外部程序可以嵌入路由、认证、热重载与翻译层，而无需依赖 CLI 二进制。

## 安装与导入

```bash
go get github.com/shinmentakezo07/shinway/v7/sdk/shinway
```

```go
import (
    "context"
    "errors"
    "time"

    "github.com/shinmentakezo07/shinway/v7/sdk/config"
    "github.com/shinmentakezo07/shinway/v7/sdk/shinway"
)
```

注意模块路径中的 `/v7`。

## 最小嵌入

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil { panic(err) }

svc, err := shinway.NewBuilder().
    WithConfig(cfg).
    WithConfigPath("config.yaml"). // 绝对路径或相对工作目录
    Build()
if err != nil { panic(err) }

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
    panic(err)
}
```

服务负责配置/认证监听、后台令牌刷新与优雅关闭。取消 context 即可停止。

## 服务端选项（中间件、路由、日志）

服务通过 `WithServerOptions` 接受选项：

```go
import (
    "github.com/shinmentakezo07/shinway/v7/sdk/api"
    "github.com/shinmentakezo07/shinway/v7/sdk/logging"
)

svc, _ := shinway.NewBuilder().
  WithConfig(cfg).
  WithConfigPath("config.yaml").
  WithServerOptions(
    // 添加全局中间件
    api.WithMiddleware(func(c *gin.Context) { c.Header("X-Embed", "1"); c.Next() }),
    // 早期调整 gin 引擎（CORS、可信代理等）
    api.WithEngineConfigurator(func(e *gin.Engine) { e.ForwardedByClientIP = true }),
    // 在默认路由之后添加自定义路由
    api.WithRouterConfigurator(func(e *gin.Engine, _ *handlers.BaseAPIHandler, _ *config.Config) {
      e.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
    }),
    // 覆盖请求日志写入器/目录
    api.WithRequestLoggerFactory(func(cfg *config.Config, cfgPath string) logging.RequestLogger {
      return logging.NewFileRequestLogger(true, "logs", filepath.Dir(cfgPath))
    }),
  ).
  Build()
```

这些选项与 CLI 服务器内部使用的选项一致，并从公开的 `sdk/api` 包重新导出（`api.ServerOption`、`api.WithMiddleware`、`api.WithEngineConfigurator`、`api.WithRouterConfigurator`、`api.WithRequestLoggerFactory`、`api.WithLocalManagementPassword`）。请求日志接口与文件日志器从 `sdk/logging` 导出。

## 管理 API（嵌入时）

- 仅当 `config.yaml` 中设置了 `remote-management.secret-key`，或通过 `api.WithLocalManagementPassword` 配置了本地管理密码时，才会挂载管理端点。
- 远程访问还需要 `remote-management.allow-remote: true`。
- 嵌入式服务器在配置端口下通过 `/v0/management` 暴露管理端点（请求日志：GET/PUT `/v0/management/request-log`，调试：`/v0/management/debug`，按 ID 查询：`/v0/management/request-log-by-id/:id`）。

## 使用核心认证管理器

服务使用核心 `auth.Manager` 完成选择、执行与自动刷新。嵌入时可以提供自定义管理器以定制传输层或钩子：

```go
core := coreauth.NewManager(sdkAuth.GetTokenStore(), nil, nil)
core.SetRoundTripperProvider(myRTProvider) // 按 auth 提供 *http.Transport

svc, _ := shinway.NewBuilder().
    WithConfig(cfg).
    WithConfigPath("config.yaml").
    WithCoreAuthManager(core).
    Build()
```

实现自定义的按 auth 传输：

```go
type myRTProvider struct{}
func (myRTProvider) RoundTripperFor(a *coreauth.Auth) http.RoundTripper {
    if a == nil || a.ProxyURL == "" { return nil }
    u, _ := url.Parse(a.ProxyURL)
    return &http.Transport{ Proxy: http.ProxyURL(u) }
}
```

管理器提供编程式执行：

```go
// 非流式
resp, err := core.Execute(ctx, []string{"gemini"}, req, opts)

// 流式
stream, err := core.ExecuteStream(ctx, []string{"gemini"}, req, opts)
for chunk := range stream.Chunks { /* ... */ }
```

注意：运行 `Service` 时内置的提供商执行器会自动接线。若要在没有 HTTP 服务器的情况下独立使用 `Manager`，必须自行注册实现 `auth.ProviderExecutor` 的执行器。

## 自定义客户端来源

如果凭据不在本地文件系统中，可以替换默认加载器：

```go
type memoryTokenProvider struct{}
func (p *memoryTokenProvider) Load(ctx context.Context, cfg *config.Config) (*shinway.TokenClientResult, error) {
    // 从内存/远程存储加载并返回计数
    return &shinway.TokenClientResult{}, nil
}

svc, _ := shinway.NewBuilder().
  WithConfig(cfg).
  WithConfigPath("config.yaml").
  WithTokenClientProvider(&memoryTokenProvider{}).
  WithAPIKeyClientProvider(shinway.NewAPIKeyClientProvider()).
  Build()
```

## 钩子

在不改动内部实现的前提下观察生命周期：

```go
hooks := shinway.Hooks{
  OnBeforeStart: func(cfg *config.Config) { log.Infof("starting on :%d", cfg.Port) },
  OnAfterStart:  func(s *shinway.Service) { log.Info("ready") },
}
svc, _ := shinway.NewBuilder().WithConfig(cfg).WithConfigPath("config.yaml").WithHooks(hooks).Build()
```

## 关闭

`Run` 内部 defer 了 `Shutdown`，因此取消父 context 即可。手动停止：

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = svc.Shutdown(ctx)
```

## 备注

- 热重载：`config.yaml` 与认证存储的变更会被自动感知。
- 请求日志可通过管理 API 在运行时开关。
- 嵌入式服务器同样支持 Gemini Web 特性（`gemini-web.*`）与本地管理面板。
- 自定义执行器与翻译器参见 `docs/sdk-advanced.md`；入站请求认证参见 `docs/sdk-access.md`；认证更新队列契约参见 `docs/sdk-watcher.md`。
