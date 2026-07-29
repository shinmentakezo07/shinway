package executor

import (
	"context"
	"net/http"
	"strings"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	nvidiaapplier "github.com/shinmentakezo07/shinway/v7/internal/thinking/provider/nvidia"
	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NVIDIAProviderKey identifies the NVIDIA NIM provider in routing, config, and registry.
const NVIDIAProviderKey = "nvidia"

// NVIDIAExecutor wraps the generic OpenAI-compatible executor with NVIDIA-NIM
// specific request fixes:
//
//   - Normalize "developer" role to "system" (NIM returns 500s when developer is
//     combined with chat_template_kwargs).
//   - Flatten content: []{type:"text", text:...} arrays into plain strings —
//     older / smaller NIM chat templates reject the array shape.
//   - Rename max_completion_tokens to max_tokens (the NIM gateway accepts
//     max_tokens reliably).
//   - Translate model reasoning intent (suffix, body level/budget, or the
//     provider default) into the per-model-family chat_template_kwargs expected
//     by NIM chat templates.
//   - Strip top-level reasoning_effort unless the model profile whitelists it
//     (Kimi); everywhere else the intent is expressed through chat_template_kwargs.
//
// The rewriting happens in a post-translation hook so it runs on the OpenAI-
// format body regardless of whether the client spoke OpenAI chat completions,
// Claude messages, Gemini, or Codex responses. The generic inner executor's
// thinking pass is then disabled (model suffix stripped) so it cannot re-apply
// a plain reasoning_effort on top of the NVIDIA kwargs.
type NVIDIAExecutor struct {
	inner *OpenAICompatExecutor
	cfg   *config.Config
}

// NewNVIDIAExecutor creates a new NVIDIA NIM executor that delegates wire work
// to the shared OpenAI-compatible executor, wiring the NVIDIA post-translation
// hook into the inner executor's pipeline.
func NewNVIDIAExecutor(cfg *config.Config) *NVIDIAExecutor {
	e := &NVIDIAExecutor{cfg: cfg}
	e.inner = NewOpenAICompatExecutorWithHook(NVIDIAProviderKey, cfg, e.translateHook)
	return e
}

// Identifier implements shinwayauth.ProviderExecutor.
func (e *NVIDIAExecutor) Identifier() string { return NVIDIAProviderKey }

// PrepareRequest implements shinwayauth.ProviderExecutor.
func (e *NVIDIAExecutor) PrepareRequest(req *http.Request, auth *shinwayauth.Auth) error {
	return e.inner.PrepareRequest(req, auth)
}

// HttpRequest implements shinwayauth.ProviderExecutor.
func (e *NVIDIAExecutor) HttpRequest(ctx context.Context, auth *shinwayauth.Auth, req *http.Request) (*http.Response, error) {
	return e.inner.HttpRequest(ctx, auth, req)
}

// Execute implements shinwayauth.ProviderExecutor.
func (e *NVIDIAExecutor) Execute(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	req.Model = e.effectiveModel(req.Model)
	return e.inner.Execute(ctx, auth, req, opts)
}

// ExecuteStream implements shinwayauth.ProviderExecutor.
func (e *NVIDIAExecutor) ExecuteStream(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (*shinwayexecutor.StreamResult, error) {
	req.Model = e.effectiveModel(req.Model)
	return e.inner.ExecuteStream(ctx, auth, req, opts)
}

// effectiveModel strips any reasoning suffix from the requested model. The
// NVIDIA translate hook has already encoded the intent by the time the inner
// executor would otherwise re-interpret the suffix, so we hand the inner
// executor a clean model name and let it simply relay.
func (e *NVIDIAExecutor) effectiveModel(model string) string {
	if parsed := thinking.ParseSuffix(model); parsed.HasSuffix {
		return parsed.ModelName
	}
	return model
}

// translateHook runs on the OpenAI-format body produced by the inner
// OpenAICompatExecutor (after anthropic/gemini/... -> openai translation) and
// applies the full NVIDIA rewrite: thinking kwargs, developer->system, content
// flattening, max_tokens rename, and top-level reasoning_effort stripping.
func (e *NVIDIAExecutor) translateHook(translated []byte, model string) []byte {
	if len(translated) == 0 || !gjson.ValidBytes(translated) {
		return translated
	}

	result := translated
	baseModel := thinking.ParseSuffix(model).ModelName
	profile := nvidiaapplier.Lookup(baseModel)

	// 0. Apply NVIDIA-specific thinking translation (suffix + body intent).
	thinkingApplied, err := thinking.ApplyThinking(result, model, "openai", NVIDIAProviderKey, NVIDIAProviderKey)
	if err == nil {
		result = thinkingApplied
	}

	// 1. Normalize "developer" role to "system".
	result = replaceDeveloperRole(result)

	// 2. Flatten all-text content arrays into plain strings.
	result = flattenTextContent(result)

	// 3. Rename max_completion_tokens -> max_tokens.
	if v := gjson.GetBytes(result, "max_completion_tokens"); v.Exists() {
		if !gjson.GetBytes(result, "max_tokens").Exists() {
			result, _ = sjson.SetBytes(result, "max_tokens", v.Value())
		}
		result, _ = sjson.DeleteBytes(result, "max_completion_tokens")
	}

	// 4. Strip top-level reasoning_effort unless the model profile wants it.
	if gjson.GetBytes(result, "reasoning_effort").Exists() && !profile.SendTopLevelEffort {
		result, _ = sjson.DeleteBytes(result, "reasoning_effort")
	}

	// 5. Inject the default thinking kwargs when the caller did not specify any.
	//
	//    This covers plain requests from every client shape: the suffix parser
	//    found no explicit suffix, the body carries no chat_template_kwargs and
	//    no reasoning_effort, and the provider default is enabled.
	hasSuffix := thinking.ParseSuffix(model).HasSuffix
	hasBodyKwargs := gjson.GetBytes(result, "chat_template_kwargs").Exists()
	hasBodyTopEffort := gjson.GetBytes(result, "reasoning_effort").Exists()
	if !hasSuffix && !hasBodyKwargs && !hasBodyTopEffort && e.defaultThinkingEnabled() {
		if profile.EnableKwargs != nil {
			for key, value := range profile.EnableKwargs {
				result, _ = sjson.SetBytes(result, "chat_template_kwargs."+key, value)
			}
		}
		if profile.EffortInKwargs {
			result, _ = sjson.SetBytes(result, "chat_template_kwargs.reasoning_effort", "high")
		}
	}

	return result
}

// CountTokens implements shinwayauth.ProviderExecutor. Token counting uses the
// standard OpenAI-compatible upstream endpoint; no NVIDIA-specific rewriting is
// required.
func (e *NVIDIAExecutor) CountTokens(ctx context.Context, auth *shinwayauth.Auth, req shinwayexecutor.Request, opts shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	return e.inner.CountTokens(ctx, auth, req, opts)
}

// Refresh implements shinwayauth.ProviderExecutor by delegating to the inner
// OpenAI-compatible executor (API-key based providers refresh via Home when
// supported, otherwise no-op).
func (e *NVIDIAExecutor) Refresh(ctx context.Context, auth *shinwayauth.Auth) (*shinwayauth.Auth, error) {
	return e.inner.Refresh(ctx, auth)
}

// defaultThinkingEnabled returns the config flag (default true) or falls back to true
// when cfg is nil.
func (e *NVIDIAExecutor) defaultThinkingEnabled() bool {
	if e == nil || e.cfg == nil {
		return true
	}
	return e.cfg.NVIDIA.DefaultThinkingEnabled()
}

// replaceDeveloperRole walks messages[] and rewrites role="developer" to role="system".
func replaceDeveloperRole(body []byte) []byte {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}
	result := body
	messages.ForEach(func(idx, msg gjson.Result) bool {
		if !msg.IsObject() {
			return true
		}
		role := msg.Get("role")
		if role.Exists() && strings.EqualFold(role.String(), "developer") {
			path := "messages." + idx.String() + ".role"
			if updated, errSet := sjson.SetBytes(result, path, "system"); errSet == nil {
				result = updated
			}
		}
		return true
	})
	return result
}

// flattenTextContent collapses content arrays of the form [{type:"text", text:"..."}]
// into the equivalent plain string. This matches what every NIM chat template accepts.
func flattenTextContent(body []byte) []byte {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}
	result := body
	messages.ForEach(func(idx, msg gjson.Result) bool {
		if !msg.IsObject() {
			return true
		}
		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			return true
		}
		allText := true
		parts := make([]string, 0, len(content.Array()))
		for _, part := range content.Array() {
			if !part.IsObject() || part.Get("type").String() != "text" {
				allText = false
				break
			}
			parts = append(parts, part.Get("text").String())
		}
		if !allText {
			return true
		}
		path := "messages." + idx.String() + ".content"
		if updated, errSet := sjson.SetBytes(result, path, strings.Join(parts, "\n")); errSet == nil {
			result = updated
		}
		return true
	})
	return result
}
