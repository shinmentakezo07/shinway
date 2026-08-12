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

// ZenProviderKey identifies the OpenCode Zen provider in routing, config, and registry.
const ZenProviderKey = "zen"

// ZenExecutor executes OpenCode Zen requests through the shared OpenAI-compatible
// executor with a post-translation hook that normalises DeepSeek reasoning_content
// requirements. Zen (https://opencode.ai/zen/v1) exposes a standard OpenAI chat
// completions gateway; the hook ensures assistant messages carry reasoning_content
// when thinking mode is active, which DeepSeek models require.
type ZenExecutor struct {
	inner *OpenAICompatExecutor
	cfg   *config.Config
}

// NewZenExecutor creates a new OpenCode Zen executor that delegates wire work
// to the shared OpenAI-compatible executor with a DeepSeek reasoning hook.
func NewZenExecutor(cfg *config.Config) *ZenExecutor {
	e := &ZenExecutor{cfg: cfg}
	e.inner = NewOpenAICompatExecutorWithHook(ZenProviderKey, cfg, e.translateHook)
	// Apply provider-specific header defaults from config.
	if cfg != nil {
		e.inner.SetHeaderDefaults(&cfg.ZenHeaderDefaults)
	}
	return e
}

// isDeepSeekModel reports whether the model name indicates a DeepSeek model.
// Zen serves DeepSeek models under plain upstream names like "deepseek-v4-pro",
// "deepseek-v4-flash", "deepseek-v3.1", etc.
func isDeepSeekModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	// Strip thinking suffix before checking.
	if parsed := thinking.ParseSuffix(model); parsed.HasSuffix {
		model = strings.ToLower(strings.TrimSpace(parsed.ModelName))
	}
	return strings.HasPrefix(model, "deepseek")
}

// zenReasoningLevels maps the reasoning_effort labels Zen clients may request
// to the canonical labels sent upstream. The generic OpenAI thinking pipeline
// converts "max" to a token budget and back to "xhigh", and it does not
// recognize "mid" at all, so Zen re-applies these labels after the generic
// pass. "mid" is a client-facing alias: Zen's upstream expects "medium".
var zenReasoningLevels = map[string]string{
	"low":   "low",
	"mid":   "medium",
	"high":  "high",
	"xhigh": "xhigh",
	"max":   "max",
}

// applyZenReasoningLevel ensures the request carries the client-requested Zen
// reasoning level as reasoning_effort, mapped to the upstream label. It runs
// after the generic thinking application and overrides its output for the Zen
// label set:
//   - Model suffix has priority (mirrors ApplyThinking's suffix priority):
//     "(low)", "(mid)", "(high)", "(xhigh)", "(max)".
//   - Otherwise, a body reasoning_effort in the Zen label set is normalized to
//     its canonical lowercase upstream form (e.g. "MAX" -> "max", "mid" ->
//     "medium") and kept verbatim.
//
// Levels outside the set (none, auto, minimal, medium, numeric budgets) are
// left to the generic pipeline untouched.
func applyZenReasoningLevel(body []byte, model string) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}

	if parsed := thinking.ParseSuffix(model); parsed.HasSuffix {
		level := strings.ToLower(strings.TrimSpace(parsed.RawSuffix))
		if upstream, ok := zenReasoningLevels[level]; ok {
			updated, errSet := sjson.SetBytes(body, "reasoning_effort", upstream)
			if errSet != nil {
				// A failed set passes the original body through unchanged (matches openai/apply.go).
				return body
			}
			return updated
		}
	}

	effort := gjson.GetBytes(body, "reasoning_effort")
	if !effort.Exists() {
		return body
	}
	level := strings.ToLower(strings.TrimSpace(effort.String()))
	if upstream, ok := zenReasoningLevels[level]; ok {
		updated, errSet := sjson.SetBytes(body, "reasoning_effort", upstream)
		if errSet != nil {
			// A failed set passes the original body through unchanged (matches openai/apply.go).
			return body
		}
		return updated
	}
	return body
}

// translateHook runs on the OpenAI-format body produced by the inner
// OpenAICompatExecutor and applies Zen-specific normalisations:
//   - Applies thinking configuration (the hook owns this when present).
//   - Ensures the Zen reasoning labels (low/mid/high/xhigh/max) reach upstream
//     as reasoning_effort ("mid" maps to "medium"), overriding generic budget
//     round-trips.
//   - For DeepSeek models: ensures assistant messages with tool_calls carry
//     reasoning_content, which DeepSeek requires when thinking mode is active.
func (e *ZenExecutor) translateHook(translated []byte, model string) []byte {
	if len(translated) == 0 || !gjson.ValidBytes(translated) {
		return translated
	}

	// 1. Apply thinking config (hook owns this for Zen, skipping generic pass).
	// Zen uses the OpenAI-compatible reasoning_effort format. Keep Zen as the
	// registry provider key for model lookup, but use the registered OpenAI
	// applier to encode the translated payload.
	applied, err := thinking.ApplyThinking(translated, model, "openai", "openai", ZenProviderKey)
	if err == nil {
		translated = applied
	}

	// 2. Re-apply the Zen reasoning label set verbatim. The generic pipeline
	// converts "max" to a budget (128000) and back to "xhigh", and drops the
	// "mid" suffix; Zen forwards the requested label unchanged instead.
	translated = applyZenReasoningLevel(translated, model)

	// 3. For DeepSeek models, normalise reasoning_content in assistant messages.
	if isDeepSeekModel(model) {
		translated = normaliseDeepSeekReasoningContent(translated)
	}

	return translated
}

// normaliseDeepSeekReasoningContent ensures assistant messages with tool_calls
// carry reasoning_content when reasoning_effort is active. DeepSeek requires
// this field to be present (non-empty) in every assistant message when thinking
// mode is enabled; otherwise the upstream returns an invalid_request_error.
func normaliseDeepSeekReasoningContent(body []byte) []byte {
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
		// developer message is an in-turn instruction, not a new turn: OpenAI
		// tool loops occasionally re-inject a system message between a tool
		// result and the follow-up assistant tool_calls message, and the
		// follow-up is still the same tool-call turn, so the fallback must
		// survive it. Tool messages stay within the same turn and keep the
		// fallback available.
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
			fallback := deepSeekReasoningFallback(msg, hasLatestReasoning, latestReasoning)
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

// deepSeekReasoningFallback builds a fallback reasoning_content value for an
// assistant message that has tool_calls but no reasoning_content.
// Priority: latest reasoning from a prior assistant > message content text > placeholder.
func deepSeekReasoningFallback(msg gjson.Result, hasLatest bool, latest string) string {
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
func (e *ZenExecutor) Identifier() string { return ZenProviderKey }

// PrepareRequest implements shinwayauth.ProviderExecutor.
func (e *ZenExecutor) PrepareRequest(req *http.Request, auth *shinwayauth.Auth) error {
	return e.inner.PrepareRequest(req, auth)
}

// HttpRequest implements shinwayauth.ProviderExecutor.
func (e *ZenExecutor) HttpRequest(ctx context.Context, auth *shinwayauth.Auth, req *http.Request) (*http.Response, error) {
	return e.inner.HttpRequest(ctx, auth, req)
}

// Execute implements shinwayauth.ProviderExecutor.
func (e *ZenExecutor) Execute(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	if opts.Alt == "responses/compact" {
		return shinwayexecutor.Response{}, statusErr{code: http.StatusNotImplemented, msg: "/responses/compact not supported"}
	}
	return e.inner.Execute(ctx, auth, req, opts)
}

// ExecuteStream implements shinwayauth.ProviderExecutor.
func (e *ZenExecutor) ExecuteStream(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (*shinwayexecutor.StreamResult, error) {
	return e.inner.ExecuteStream(ctx, auth, req, opts)
}

// CountTokens implements shinwayauth.ProviderExecutor. Token counting uses the
// standard OpenAI-compatible upstream endpoint.
func (e *ZenExecutor) CountTokens(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	return e.inner.CountTokens(ctx, auth, req, opts)
}

// Refresh implements shinwayauth.ProviderExecutor by delegating to the inner
// OpenAI-compatible executor (API-key based providers refresh via Home when
// supported, otherwise no-op).
func (e *ZenExecutor) Refresh(ctx context.Context, auth *shinwayauth.Auth) (*shinwayauth.Auth, error) {
	return e.inner.Refresh(ctx, auth)
}
