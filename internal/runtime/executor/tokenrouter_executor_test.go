package executor

import (
	"context"
	"net/http"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	"github.com/tidwall/gjson"
)

func TestTokenRouterExecutorRejectsResponsesCompact(t *testing.T) {
	executor := NewTokenRouterExecutor(&config.Config{})

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

func TestTokenRouterExecutorTranslateHookAppliesOpenAIReasoningEffort(t *testing.T) {
	executor := NewTokenRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek/deepseek-v4-pro-0813","reasoning_effort":"high"}`), "deepseek/deepseek-v4-pro-0813")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestTokenRouterExecutorTranslateHookAppliesThinkingSuffix(t *testing.T) {
	executor := NewTokenRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek/deepseek-v4-pro-0813"}`), "deepseek/deepseek-v4-pro-0813(high)")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestTokenRouterExecutorTranslateHookAddsReasoningContentToDeepSeekToolHistory(t *testing.T) {
	executor := NewTokenRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek/deepseek-v4-pro-0813",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"Use the tool."},
    {"role":"assistant","content":"I need a tool.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"result"}
  ]
}`), "deepseek/deepseek-v4-pro-0813")

	if got := gjson.GetBytes(translated, "messages.1.reasoning_content").String(); got != "I need a tool." {
		t.Fatalf("tool-call reasoning_content = %q, want fallback assistant content", got)
	}
}

func TestIsTokenRouterDeepSeekModel(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"deepseek/deepseek-v4-pro-0813", true},
		{"deepseek/deepseek-v3.1", true},
		{"deepseek-v4-pro", true},
		{"DEEPSEEK/deepseek-v4-pro(high)", true},
		{"anthropic/claude-sonnet-4.6", false},
		{"openai/gpt-5.6-sol", false},
		{"qwen/qwen3.8-max", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isTokenRouterDeepSeekModel(tc.model); got != tc.want {
			t.Errorf("isTokenRouterDeepSeekModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}
