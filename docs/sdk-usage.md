# Shinway SDK Guide

The `sdk/shinway` module exposes the proxy as a reusable Go library so external programs can embed the routing, authentication, hot‑reload, and translation layers without depending on the CLI binary.

## Install & Import

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

Note the `/v7` module path.

## Minimal Embed

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil { panic(err) }

svc, err := shinway.NewBuilder().
    WithConfig(cfg).
    WithConfigPath("config.yaml"). // absolute or working-dir relative
    Build()
if err != nil { panic(err) }

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
    panic(err)
}
```

The service manages config/auth watching, background token refresh, and graceful shutdown. Cancel the context to stop it.

## Server Options (middleware, routes, logs)

The server accepts options via `WithServerOptions`:

```go
import (
    "github.com/shinmentakezo07/shinway/v7/sdk/api"
    "github.com/shinmentakezo07/shinway/v7/sdk/logging"
)

svc, _ := shinway.NewBuilder().
  WithConfig(cfg).
  WithConfigPath("config.yaml").
  WithServerOptions(
    // Add global middleware
    api.WithMiddleware(func(c *gin.Context) { c.Header("X-Embed", "1"); c.Next() }),
    // Tweak gin engine early (CORS, trusted proxies, etc.)
    api.WithEngineConfigurator(func(e *gin.Engine) { e.ForwardedByClientIP = true }),
    // Add your own routes after defaults
    api.WithRouterConfigurator(func(e *gin.Engine, _ *handlers.BaseAPIHandler, _ *config.Config) {
      e.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
    }),
    // Override request log writer/dir
    api.WithRequestLoggerFactory(func(cfg *config.Config, cfgPath string) logging.RequestLogger {
      return logging.NewFileRequestLogger(true, "logs", filepath.Dir(cfgPath))
    }),
  ).
  Build()
```

These options mirror the internals used by the CLI server and are re-exported from the public `sdk/api` package (`api.ServerOption`, `api.WithMiddleware`, `api.WithEngineConfigurator`, `api.WithRouterConfigurator`, `api.WithRequestLoggerFactory`, `api.WithLocalManagementPassword`). The request-logger interface and file logger are exported from `sdk/logging`.

## Management API (when embedded)

- Management endpoints are mounted only when `remote-management.secret-key` is set in `config.yaml`, or when a local management password is configured via `api.WithLocalManagementPassword`.
- Remote access additionally requires `remote-management.allow-remote: true`.
- Your embedded server exposes management endpoints under `/v0/management` on the configured port (request log: GET/PUT `/v0/management/request-log`, debug: `/v0/management/debug`, and per-request logs at `/v0/management/request-log-by-id/:id`).

## Using the Core Auth Manager

The service uses a core `auth.Manager` for selection, execution, and auto‑refresh. When embedding, you can provide your own manager to customize transports or hooks:

```go
core := coreauth.NewManager(sdkAuth.GetTokenStore(), nil, nil)
core.SetRoundTripperProvider(myRTProvider) // per‑auth *http.Transport

svc, _ := shinway.NewBuilder().
    WithConfig(cfg).
    WithConfigPath("config.yaml").
    WithCoreAuthManager(core).
    Build()
```

Implement a custom per‑auth transport:

```go
type myRTProvider struct{}
func (myRTProvider) RoundTripperFor(a *coreauth.Auth) http.RoundTripper {
    if a == nil || a.ProxyURL == "" { return nil }
    u, _ := url.Parse(a.ProxyURL)
    return &http.Transport{ Proxy: http.ProxyURL(u) }
}
```

Programmatic execution is available on the manager:

```go
// Non‑streaming
resp, err := core.Execute(ctx, []string{"gemini"}, req, opts)

// Streaming
stream, err := core.ExecuteStream(ctx, []string{"gemini"}, req, opts)
for chunk := range stream.Chunks { /* ... */ }
```

Note: Built‑in provider executors are wired automatically when you run the `Service`. If you want to use `Manager` stand‑alone without the HTTP server, you must register your own executors that implement `auth.ProviderExecutor`.

## Custom Client Sources

Replace the default loaders if your creds live outside the local filesystem:

```go
type memoryTokenProvider struct{}
func (p *memoryTokenProvider) Load(ctx context.Context, cfg *config.Config) (*shinway.TokenClientResult, error) {
    // Populate from memory/remote store and return counts
    return &shinway.TokenClientResult{}, nil
}

svc, _ := shinway.NewBuilder().
  WithConfig(cfg).
  WithConfigPath("config.yaml").
  WithTokenClientProvider(&memoryTokenProvider{}).
  WithAPIKeyClientProvider(shinway.NewAPIKeyClientProvider()).
  Build()
```

## Hooks

Observe lifecycle without patching internals:

```go
hooks := shinway.Hooks{
  OnBeforeStart: func(cfg *config.Config) { log.Infof("starting on :%d", cfg.Port) },
  OnAfterStart:  func(s *shinway.Service) { log.Info("ready") },
}
svc, _ := shinway.NewBuilder().WithConfig(cfg).WithConfigPath("config.yaml").WithHooks(hooks).Build()
```

## Shutdown

`Run` defers `Shutdown`, so cancelling the parent context is enough. To stop manually:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = svc.Shutdown(ctx)
```

## Notes

- Hot reload: changes to `config.yaml` and the auth store are picked up automatically.
- Request logging can be toggled at runtime via the Management API.
- Gemini Web features (`gemini-web.*`) and the local management panel are honored in the embedded server.
- See `docs/sdk-advanced.md` for custom executors and translators, `docs/sdk-access.md` for inbound request authentication, and `docs/sdk-watcher.md` for the auth update queue contract.
