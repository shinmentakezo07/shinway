package pipeline

import (
	"context"
	"net/http"

	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	sdktranslator "github.com/shinmentakezo07/shinway/v7/sdk/translator"
)

// Context encapsulates execution state shared across middleware, translators, and executors.
type Context struct {
	// Request encapsulates the provider facing request payload.
	Request shinwayexecutor.Request
	// Options carries execution flags (streaming, headers, etc.).
	Options shinwayexecutor.Options
	// Auth references the credential selected for execution.
	Auth *shinwayauth.Auth
	// Translator represents the pipeline responsible for schema adaptation.
	Translator *sdktranslator.Pipeline
	// HTTPClient allows middleware to customise the outbound transport per request.
	HTTPClient *http.Client
}

// Hook captures middleware callbacks around execution.
type Hook interface {
	BeforeExecute(ctx context.Context, execCtx *Context)
	AfterExecute(ctx context.Context, execCtx *Context, resp shinwayexecutor.Response, err error)
	OnStreamChunk(ctx context.Context, execCtx *Context, chunk shinwayexecutor.StreamChunk)
}

// HookFunc aggregates optional hook implementations.
type HookFunc struct {
	Before func(context.Context, *Context)
	After  func(context.Context, *Context, shinwayexecutor.Response, error)
	Stream func(context.Context, *Context, shinwayexecutor.StreamChunk)
}

// BeforeExecute implements Hook.
func (h HookFunc) BeforeExecute(ctx context.Context, execCtx *Context) {
	if h.Before != nil {
		h.Before(ctx, execCtx)
	}
}

// AfterExecute implements Hook.
func (h HookFunc) AfterExecute(ctx context.Context, execCtx *Context, resp shinwayexecutor.Response, err error) {
	if h.After != nil {
		h.After(ctx, execCtx, resp, err)
	}
}

// OnStreamChunk implements Hook.
func (h HookFunc) OnStreamChunk(ctx context.Context, execCtx *Context, chunk shinwayexecutor.StreamChunk) {
	if h.Stream != nil {
		h.Stream(ctx, execCtx, chunk)
	}
}

// RoundTripperProvider allows injection of custom HTTP transports per auth entry.
type RoundTripperProvider interface {
	RoundTripperFor(auth *shinwayauth.Auth) http.RoundTripper
}
