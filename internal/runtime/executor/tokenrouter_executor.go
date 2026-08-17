package executor

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// TokenRouterProviderKey identifies the TokenRouter provider in routing, config, and registry.
const TokenRouterProviderKey = "tokenrouter"

// TokenRouterExecutor executes TokenRouter requests through the shared
// OpenAI-compatible executor with a post-translation hook that normalises
// DeepSeek reasoning_content requirements. TokenRouter
// (https://api.tokenrouter.com/v1) is a unified multi-model OpenAI-compatible
// gateway: one API key and one base URL route to 50+ upstream models whose IDs
// carry a provider prefix (e.g. "anthropic/claude-sonnet-4.6",
// "deepseek/deepseek-v4-pro-0813", "openai/gpt-5.6-sol"). The hook ensures
// assistant messages carry reasoning_content when thinking mode is active for
// DeepSeek models, which the DeepSeek upstream requires.
type TokenRouterExecutor struct {
	inner *OpenAICompatExecutor
	cfg   *config.Config
}

// NewTokenRouterExecutor creates a new TokenRouter executor that delegates wire
// work to the shared OpenAI-compatible executor with a DeepSeek reasoning hook.
func NewTokenRouterExecutor(cfg *config.Config) *TokenRouterExecutor {
	e := &TokenRouterExecutor{cfg: cfg}
	e.inner = NewOpenAICompatExecutorWithHook(TokenRouterProviderKey, cfg, e.translateHook)
	return e
}

// isTokenRouterDeepSeekModel reports whether the model name indicates a
// DeepSeek model. TokenRouter serves DeepSeek models under provider-prefixed
// names like "deepseek/deepseek-v4-pro-0813", so the provider prefix is
// stripped before matching.
func isTokenRouterDeepSeekModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	// Strip thinking suffix before checking.
	if parsed := thinking.ParseSuffix(model); parsed.HasSuffix {
		model = strings.ToLower(strings.TrimSpace(parsed.ModelName))
	}
	// Strip the provider prefix (e.g. "deepseek/deepseek-v4-pro").
	if idx := strings.Index(model, "/"); idx >= 0 {
		model = model[idx+1:]
	}
	return strings.HasPrefix(model, "deepseek")
}

// translateHook runs on the OpenAI-format body produced by the inner
// OpenAICompatExecutor and applies TokenRouter-specific normalisations:
//   - Applies thinking configuration (the hook owns this when present).
//   - For DeepSeek models: ensures assistant messages with tool_calls carry
//     reasoning_content, which DeepSeek requires when thinking mode is active.
func (e *TokenRouterExecutor) translateHook(translated []byte, model string) []byte {
	if len(translated) == 0 || !gjson.ValidBytes(translated) {
		return translated
	}

	// 1. Apply thinking config (hook owns this for TokenRouter, skipping the
	// generic pass). TokenRouter uses the OpenAI-compatible reasoning_effort
	// format. Keep TokenRouter as the registry provider key for model lookup,
	// but use the registered OpenAI applier to encode the translated payload.
	applied, err := thinking.ApplyThinking(translated, model, "openai", "openai", TokenRouterProviderKey)
	if err == nil {
		translated = applied
	}

	// 2. For DeepSeek models, normalise reasoning_content in assistant messages.
	if isTokenRouterDeepSeekModel(model) {
		translated = normaliseTokenRouterDeepSeekReasoningContent(translated)
	}

	return translated
}

// normaliseTokenRouterDeepSeekReasoningContent ensures assistant messages with
// tool_calls carry reasoning_content when reasoning_effort is active. DeepSeek
// requires this field to be present (non-empty) in every assistant message when
// thinking mode is enabled; otherwise the upstream returns an
// invalid_request_error.
func normaliseTokenRouterDeepSeekReasoningContent(body []byte) []byte {
	// Only act when reasoning_effort is explicitly set.
	reasoningEffort := gjson.GetBytes(body, "reasoning_effort")
	if !reasoningEffort.Exists() || reasoningEffort.String() == "none" {
		return body
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}

	type messagePatch struct {
		index int
		value string
	}

	msgs := messages.Array()
	patches := make([]messagePatch, 0, len(msgs))
	latestReasoning := ""
	hasLatestReasoning := false

	for idx, msg := range msgs {
		if !msg.IsObject() {
			continue
		}
		role := strings.TrimSpace(msg.Get("role").String())

		// A user message starts a new turn. Reasoning observed in an earlier
		// turn must not leak into tool-call messages of later turns, so the
		// carried fallback state is reset only at user boundaries. A system or
		// developer message is an in-turn instruction, not a new turn, so the
		// fallback must survive it. Tool messages stay within the same turn and
		// keep the fallback available.
		if role == "user" {
			latestReasoning = ""
			hasLatestReasoning = false
			continue
		}
		if role != "assistant" {
			continue
		}

		reasoning := msg.Get("reasoning_content")
		if reasoning.Exists() && strings.TrimSpace(reasoning.String()) != "" {
			latestReasoning = reasoning.String()
			hasLatestReasoning = true
			// Already has reasoning_content, nothing to patch.
			continue
		}

		toolCalls := msg.Get("tool_calls")
		hasToolCalls := toolCalls.Exists() && toolCalls.IsArray() && len(toolCalls.Array()) > 0

		if hasToolCalls {
			// DeepSeek requires reasoning_content when tool_calls are present
			// and thinking mode is active.
			fallback := tokenRouterDeepSeekReasoningFallback(msg, hasLatestReasoning, latestReasoning)
			patches = append(patches, messagePatch{index: idx, value: fallback})
		}
	}

	if len(patches) == 0 {
		return body
	}

	result := body
	for _, patch := range patches {
		path := fmt.Sprintf("messages.%d.reasoning_content", patch.index)
		updated, errSet := sjson.SetBytes(result, path, patch.value)
		if errSet != nil {
			continue
		}
		result = updated
	}
	return result
}

// tokenRouterDeepSeekReasoningFallback builds a fallback reasoning_content
// value for an assistant message that has tool_calls but no reasoning_content.
// Priority: latest reasoning from a prior assistant > message content text > placeholder.
func tokenRouterDeepSeekReasoningFallback(msg gjson.Result, hasLatest bool, latest string) string {
	if hasLatest && strings.TrimSpace(latest) != "" {
		return latest
	}

	content := msg.Get("content")
	if content.Type == gjson.String {
		if text := strings.TrimSpace(content.String()); text != "" {
			return text
		}
	}
	if content.IsArray() {
		var parts []string
		for _, item := range content.Array() {
			text := strings.TrimSpace(item.Get("text").String())
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	return "[reasoning unavailable]"
}

// Identifier implements shinwayauth.ProviderExecutor.
func (e *TokenRouterExecutor) Identifier() string { return TokenRouterProviderKey }

// PrepareRequest implements shinwayauth.ProviderExecutor.
func (e *TokenRouterExecutor) PrepareRequest(req *http.Request, auth *shinwayauth.Auth) error {
	return e.inner.PrepareRequest(req, auth)
}

// HttpRequest implements shinwayauth.ProviderExecutor.
func (e *TokenRouterExecutor) HttpRequest(ctx context.Context, auth *shinwayauth.Auth, req *http.Request) (*http.Response, error) {
	return e.inner.HttpRequest(ctx, auth, req)
}

// Execute implements shinwayauth.ProviderExecutor.
func (e *TokenRouterExecutor) Execute(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	if opts.Alt == "responses/compact" {
		return shinwayexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "/responses/compact not supported"}
	}
	return e.inner.Execute(ctx, auth, req, opts)
}

// ExecuteStream implements shinwayauth.ProviderExecutor.
func (e *TokenRouterExecutor) ExecuteStream(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (*shinwayexecutor.StreamResult, error) {
	return e.inner.ExecuteStream(ctx, auth, req, opts)
}

// CountTokens implements shinwayauth.ProviderExecutor. Token counting uses the
// standard OpenAI-compatible upstream endpoint.
func (e *TokenRouterExecutor) CountTokens(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	return e.inner.CountTokens(ctx, auth, req, opts)
}

// Refresh implements shinwayauth.ProviderExecutor by delegating to the inner
// OpenAI-compatible executor (API-key based providers refresh via Home when
// supported, otherwise no-op).
func (e *TokenRouterExecutor) Refresh(ctx context.Context, auth *shinwayauth.Auth) (*shinwayauth.Auth, error) {
	return e.inner.Refresh(ctx, auth)
}
