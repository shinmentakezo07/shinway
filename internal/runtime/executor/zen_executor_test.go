package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	sdktranslator "github.com/shinmentakezo07/shinway/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestZenExecutorRejectsResponsesCompact(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	_, err := executor.Execute(context.Background(), nil, shinwayexecutor.Request{}, shinwayexecutor.Options{
		Alt: "responses/compact",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported-operation error")
	}

	status, ok := err.(statusErr)
	if !ok {
		t.Fatalf("Execute() error = %T, want statusErr", err)
	}
	if status.code != http.StatusNotImplemented {
		t.Fatalf("Execute() status = %d, want %d", status.code, http.StatusNotImplemented)
	}
}

func TestZenExecutorTranslateHookAppliesOpenAIReasoningEffort(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek-v4-pro","reasoning_effort":"high"}`), "deepseek-v4-pro")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestZenExecutorTranslateHookAppliesThinkingSuffix(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek-v4-pro"}`), "deepseek-v4-pro(high)")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestZenExecutorTranslateHookAddsReasoningContentToDeepSeekToolHistory(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek-v4-flash-free",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"Use the tool."},
    {"role":"assistant","content":"I need a tool.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"result"}
  ]
}`), "deepseek-v4-flash-free")

	if got := gjson.GetBytes(translated, "messages.1.reasoning_content").String(); got != "I need a tool." {
		t.Fatalf("tool-call reasoning_content = %q, want fallback assistant content", got)
	}
}

func TestZenExecutorTranslateHookDoesNotLeakReasoningAcrossTurns(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek-v4-flash-free",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"Turn one."},
    {"role":"assistant","content":"","reasoning_content":"R1-first-turn","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"result"},
    {"role":"assistant","tool_calls":[{"id":"call_2","type":"function","function":{"name":"edit","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_2","content":"done"},
    {"role":"user","content":"Turn two starts here."},
    {"role":"assistant","tool_calls":[{"id":"call_3","type":"function","function":{"name":"run_tests","arguments":"{}"}}]}
  ]
}`), "deepseek-v4-flash-free")

	// Same-turn tool-call message keeps the fallback from the preceding assistant message.
	if got := gjson.GetBytes(translated, "messages.3.reasoning_content").String(); got != "R1-first-turn" {
		t.Fatalf("same-turn tool-call reasoning_content = %q, want preceding turn reasoning", got)
	}
	// A new turn must not inherit reasoning from the previous turn.
	if got := gjson.GetBytes(translated, "messages.6.reasoning_content").String(); got == "R1-first-turn" {
		t.Fatalf("cross-turn reasoning leaked: messages.6.reasoning_content = %q", got)
	} else if got != "[reasoning unavailable]" {
		t.Fatalf("cross-turn tool-call reasoning_content = %q, want placeholder fallback", got)
	}
}

func TestZenExecutorTranslateHookDoesNotLeakReasoningAcrossUserBoundary(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek-v4-flash-free",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"First."},
    {"role":"assistant","content":"","reasoning_content":"R1","tool_calls":[{"id":"call_1","type":"function","function":{"name":"a","arguments":"{}"}}]},
    {"role":"user","content":"Second."},
    {"role":"assistant","content":"","reasoning_content":"R2","tool_calls":[{"id":"call_2","type":"function","function":{"name":"b","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_2","content":"ok"},
    {"role":"assistant","tool_calls":[{"id":"call_3","type":"function","function":{"name":"c","arguments":"{}"}}]}
  ]
}`), "deepseek-v4-flash-free")

	if got := gjson.GetBytes(translated, "messages.5.reasoning_content").String(); got != "R2" {
		t.Fatalf("same-turn tool-call reasoning_content = %q, want R2 (latest reasoning within the turn)", got)
	}
}

func TestZenExecutorTranslateHookKeepsReasoningAcrossMidTurnSystemMessage(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek-v4-flash-free",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"Use the tool."},
    {"role":"assistant","content":"","reasoning_content":"R1","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"result"},
    {"role":"system","content":"Reminder: finish the task."},
    {"role":"assistant","tool_calls":[{"id":"call_2","type":"function","function":{"name":"edit","arguments":"{}"}}]}
  ]
}`), "deepseek-v4-flash-free")

	// The system message is an in-turn instruction, not a turn boundary: the
	// follow-up assistant tool_calls message is still the same tool-call turn,
	// so it keeps the reasoning carried from the earlier assistant message.
	if got := gjson.GetBytes(translated, "messages.4.reasoning_content").String(); got != "R1" {
		t.Fatalf("tool-call reasoning_content after mid-turn system = %q, want R1 (same turn)", got)
	}
}

func TestZenExecutorTranslateHookForwardsZenReasoningLevels(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	tests := []struct {
		name  string
		body  string
		model string
		want  string
	}{
		{
			name:  "bodymax",
			body:  `{"model":"glm-5.2","reasoning_effort":"max","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2",
			want:  "max",
		},
		{
			name:  "bodyxhigh",
			body:  `{"model":"glm-5.2","reasoning_effort":"xhigh","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2",
			want:  "xhigh",
		},
		{
			name:  "bodymidbecomesmedium",
			body:  `{"model":"glm-5.2","reasoning_effort":"mid","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2",
			want:  "medium",
		},
		{
			name:  "bodymediumuntouched",
			body:  `{"model":"glm-5.2","reasoning_effort":"medium","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2",
			want:  "medium",
		},
		{
			name:  "bodyuppercase",
			body:  `{"model":"glm-5.2","reasoning_effort":"MAX","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2",
			want:  "max",
		},
		{
			name:  "suffixlow",
			body:  `{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2(low)",
			want:  "low",
		},
		{
			name:  "suffixmidbecomesmedium",
			body:  `{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2(mid)",
			want:  "medium",
		},
		{
			name:  "suffixmax",
			body:  `{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2(max)",
			want:  "max",
		},
		{
			name:  "suffixxhigh",
			body:  `{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2(xhigh)",
			want:  "xhigh",
		},
		{
			name:  "suffixnoneuntouched",
			body:  `{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`,
			model: "glm-5.2(none)",
			want:  "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translated := executor.translateHook([]byte(tt.body), tt.model)
			got := gjson.GetBytes(translated, "reasoning_effort").String()
			if got != tt.want {
				t.Fatalf("reasoning_effort = %q, want %q (body=%s)", got, tt.want, string(translated))
			}
		})
	}
}

func TestZenExecutorTranslateHookModelSuffixBeatsBodyEffort(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"glm-5.2",
  "reasoning_effort":"high",
  "messages":[{"role":"user","content":"hi"}]
}`), "glm-5.2(max)")

	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "max" {
		t.Fatalf("reasoning_effort = %q, want max (suffix priority)", got)
	}
}

func TestZenExecutorTranslateHookNoEffortStaysAbsent(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"glm-5.2",
  "messages":[{"role":"user","content":"hi"}]
}`), "glm-5.2")

	if got := gjson.GetBytes(translated, "reasoning_effort"); got.Exists() {
		t.Fatalf("reasoning_effort unexpectedly set: %s", got.Raw)
	}
}

// zenHeaderCaptureServer starts a chat-completions test server that records the
// request headers it receives and returns a minimal OpenAI completion.
func zenHeaderCaptureServer(t *testing.T, capture *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capture = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl_1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
}

// TestZenExecutorSendsOpencodeIdentityHeaders verifies that Zen requests carry
// the same request-identity headers as the real opencode client by default:
// HTTP-Referer: https://opencode.ai/, X-Title: opencode, and an opencode
// User-Agent, instead of the generic cli-proxy-openai-compat identity.
func TestZenExecutorSendsOpencodeIdentityHeaders(t *testing.T) {
	var got http.Header
	server := zenHeaderCaptureServer(t, &got)
	defer server.Close()

	executor := NewZenExecutor(&config.Config{})
	auth := &shinwayauth.Auth{Attributes: map[string]string{
		"base_url": server.URL + "/v1",
		"api_key":  "test-key",
	}}
	_, err := executor.Execute(context.Background(), auth, shinwayexecutor.Request{
		Model:   "deepseek-v4-flash-free",
		Payload: []byte(`{"model":"deepseek-v4-flash-free","messages":[{"role":"user","content":"hi"}]}`),
	}, shinwayexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotVal := got.Get("HTTP-Referer"); gotVal != "https://opencode.ai/" {
		t.Fatalf("HTTP-Referer = %q, want %q", gotVal, "https://opencode.ai/")
	}
	if gotVal := got.Get("X-Title"); gotVal != "opencode" {
		t.Fatalf("X-Title = %q, want %q", gotVal, "opencode")
	}
	if gotVal := got.Get("User-Agent"); gotVal != zenDefaultUserAgent {
		t.Fatalf("User-Agent = %q, want %q", gotVal, zenDefaultUserAgent)
	}
}

// TestZenExecutorConfiguredHeaderDefaultsOverrideOpencode verifies that
// zen-header-defaults values take precedence over the opencode defaults.
func TestZenExecutorConfiguredHeaderDefaultsOverrideOpencode(t *testing.T) {
	var got http.Header
	server := zenHeaderCaptureServer(t, &got)
	defer server.Close()

	cfg := &config.Config{
		ZenHeaderDefaults: config.ZenHeaderDefaults{
			HTTPReferer: "https://custom.example.com/",
			XTitle:      "my-title",
			UserAgent:   "custom-agent/1.0",
		},
	}
	executor := NewZenExecutor(cfg)
	auth := &shinwayauth.Auth{Attributes: map[string]string{
		"base_url": server.URL + "/v1",
		"api_key":  "test-key",
	}}
	_, err := executor.Execute(context.Background(), auth, shinwayexecutor.Request{
		Model:   "glm-5.2",
		Payload: []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"hi"}]}`),
	}, shinwayexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotVal := got.Get("HTTP-Referer"); gotVal != "https://custom.example.com/" {
		t.Fatalf("HTTP-Referer = %q, want custom", gotVal)
	}
	if gotVal := got.Get("X-Title"); gotVal != "my-title" {
		t.Fatalf("X-Title = %q, want custom", gotVal)
	}
	if gotVal := got.Get("User-Agent"); gotVal != "custom-agent/1.0" {
		t.Fatalf("User-Agent = %q, want custom", gotVal)
	}
}

// TestZenExecutorPerKeyCustomHeaderWinsOverDefault verifies that per-key custom
// headers (zen-api-key.headers) override the opencode identity defaults.
func TestZenExecutorPerKeyCustomHeaderWinsOverDefault(t *testing.T) {
	var got http.Header
	server := zenHeaderCaptureServer(t, &got)
	defer server.Close()

	executor := NewZenExecutor(&config.Config{})
	auth := &shinwayauth.Auth{Attributes: map[string]string{
		"base_url":       server.URL + "/v1",
		"api_key":        "test-key",
		"header:X-Title": "key-title",
	}}
	_, err := executor.Execute(context.Background(), auth, shinwayexecutor.Request{
		Model:   "kimi-k2.5",
		Payload: []byte(`{"model":"kimi-k2.5","messages":[{"role":"user","content":"hi"}]}`),
	}, shinwayexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
		Stream:       false,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotVal := got.Get("X-Title"); gotVal != "key-title" {
		t.Fatalf("X-Title = %q, want per-key override %q", gotVal, "key-title")
	}
	// The default Referer must still be present when only X-Title is overridden.
	if gotVal := got.Get("HTTP-Referer"); gotVal != "https://opencode.ai/" {
		t.Fatalf("HTTP-Referer = %q, want default", gotVal)
	}
}

// TestZenExecutorStreamSendsOpencodeIdentityHeaders verifies streaming requests
// carry the opencode identity headers as well.
func TestZenExecutorStreamSendsOpencodeIdentityHeaders(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()

	executor := NewZenExecutor(&config.Config{})
	auth := &shinwayauth.Auth{Attributes: map[string]string{
		"base_url": server.URL + "/v1",
		"api_key":  "test-key",
	}}
	result, err := executor.ExecuteStream(context.Background(), auth, shinwayexecutor.Request{
		Model:   "deepseek-v4-flash-free",
		Payload: []byte(`{"model":"deepseek-v4-flash-free","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	}, shinwayexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteStream error: %v", err)
	}
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			t.Fatalf("stream error: %v", chunk.Err)
		}
	}
	if gotVal := got.Get("HTTP-Referer"); gotVal != "https://opencode.ai/" {
		t.Fatalf("stream HTTP-Referer = %q, want default", gotVal)
	}
	if gotVal := got.Get("X-Title"); gotVal != "opencode" {
		t.Fatalf("stream X-Title = %q, want default", gotVal)
	}
	if gotVal := got.Get("User-Agent"); gotVal != zenDefaultUserAgent {
		t.Fatalf("stream User-Agent = %q, want %q", gotVal, zenDefaultUserAgent)
	}
}

// TestZenExecutorPrepareRequestAppliesHeaderDefaults verifies the auth-flow
// PrepareRequest path also injects the opencode identity headers.
func TestZenExecutorPrepareRequestAppliesHeaderDefaults(t *testing.T) {
	executor := NewZenExecutor(&config.Config{})
	req := httptest.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/chat/completions", strings.NewReader(`{}`))
	err := executor.PrepareRequest(req, &shinwayauth.Auth{Attributes: map[string]string{"api_key": "test-key"}})
	if err != nil {
		t.Fatalf("PrepareRequest error: %v", err)
	}
	if gotVal := req.Header.Get("HTTP-Referer"); gotVal != "https://opencode.ai/" {
		t.Fatalf("HTTP-Referer = %q, want default", gotVal)
	}
	if gotVal := req.Header.Get("X-Title"); gotVal != "opencode" {
		t.Fatalf("X-Title = %q, want default", gotVal)
	}
}
