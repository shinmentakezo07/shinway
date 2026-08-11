# sdk/access SDK Reference

The `sdk/access` package centralizes inbound request authentication for the proxy. It offers a lightweight manager that chains credential providers, so servers can reuse the same access control logic inside or outside the CLI runtime.

## Importing

```go
import (
    sdkaccess "github.com/shinmentakezo07/shinway/v7/sdk/access"
)
```

Add the module with `go get github.com/shinmentakezo07/shinway/v7/sdk/access`.

## Provider Registry

Providers are registered globally and then attached to a `Manager` as a snapshot:

- `RegisterProvider(type, provider)` installs a pre-initialized provider instance.
- Registration order is preserved the first time each `type` is seen.
- `UnregisterProvider(type)` removes a provider by type identifier.
- `SetExclusiveProvider(type)` restricts `RegisteredProviders()` to a single key; `ClearExclusiveProvider()` removes the restriction.
- `RegisteredProviders()` returns the providers in registration order.

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

## Manager Lifecycle

```go
manager := sdkaccess.NewManager()
manager.SetProviders(sdkaccess.RegisteredProviders())
```

- `NewManager` constructs an empty manager.
- `SetProviders` replaces the provider slice using a defensive copy.
- `Providers` retrieves a snapshot that can be iterated safely from other goroutines.

If the manager itself is `nil` or no providers are configured, the call returns `nil, nil`, allowing callers to treat access control as disabled.

## Authenticating Requests

```go
result, authErr := manager.Authenticate(ctx, req)
switch {
case authErr == nil:
    // Access granted. result.Principal and result.Metadata identify the caller.
case sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeNoCredentials):
    // No credentials supplied.
case sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeInvalidCredential):
    // Credentials were supplied but are invalid.
default:
    // Provider-specific failure; propagate or map to 500.
}
```

The manager evaluates providers in order and short-circuits on the first success. Providers that return `AuthErrorCodeNotHandled` are skipped so the chain can continue. A missing-credential result is remembered while invalid-credential results take precedence at the end.

## Error Helpers

- `NewNoCredentialsError()` – no credentials were provided.
- `NewInvalidCredentialError()` – credentials were present but rejected.
- `NewNotHandledError()` – the provider does not handle this request shape.
- `NewInternalAuthError(message, cause)` – unexpected failure.
- `IsAuthErrorCode(authErr, code)` – typed error inspection.

## Provider Interface

```go
type Provider interface {
    Identifier() string
    Authenticate(ctx context.Context, r *http.Request) (*Result, *AuthError)
}
```

`Result` carries `Provider`, `Principal`, and optional `Metadata`. Returning `(nil, nil)` grants anonymous access.

## Integration with the Service

The embedded service wires the access manager automatically when `sdk/shinway`'s builder runs: it calls `SetProviders(sdkaccess.RegisteredProviders())` and configures the access-managed routes. Provide your own manager through `Builder.WithRequestAccessManager` to customize the chain.
