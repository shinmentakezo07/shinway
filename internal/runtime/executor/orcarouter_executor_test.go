package executor

import (
	"context"
	"net/http"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	"github.com/tidwall/gjson"
)

func TestOrcaRouterExecutorRejectsResponsesCompact(t *testing.T) {
	executor := NewOrcaRouterExecutor(&config.Config{})

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

func TestOrcaRouterExecutorTranslateHookAppliesOpenAIReasoningEffort(t *testing.T) {
	executor := NewOrcaRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek/deepseek-chat","reasoning_effort":"high"}`), "deepseek/deepseek-chat")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestOrcaRouterExecutorTranslateHookAppliesThinkingSuffix(t *testing.T) {
	executor := NewOrcaRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{"model":"deepseek/deepseek-chat"}`), "deepseek/deepseek-chat(high)")
	if got := gjson.GetBytes(translated, "reasoning_effort").String(); got != "high" {
		t.Fatalf("reasoning_effort = %q, want high", got)
	}
}

func TestOrcaRouterExecutorTranslateHookAddsReasoningContentToDeepSeekToolHistory(t *testing.T) {
	executor := NewOrcaRouterExecutor(&config.Config{})

	translated := executor.translateHook([]byte(`{
  "model":"deepseek/deepseek-chat",
  "reasoning_effort":"high",
  "messages":[
    {"role":"user","content":"Use the tool."},
    {"role":"assistant","content":"I need a tool.","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"result"}
  ]
}`), "deepseek/deepseek-chat")

	if got := gjson.GetBytes(translated, "messages.1.reasoning_content").String(); got != "I need a tool." {
		t.Fatalf("tool-call reasoning_content = %q, want fallback assistant content", got)
	}
}

func TestIsOrcaRouterDeepSeekModel(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{"deepseek/deepseek-chat", true},
		{"deepseek/deepseek-reasoner", true},
		{"deepseek-v3", true},
		{"DEEPSEEK/deepseek-chat(high)", true},
		{"anthropic/claude-sonnet-4.6", false},
		{"openai/gpt-4o-mini", false},
		{"gemini/gemini-2.5-pro", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isOrcaRouterDeepSeekModel(tc.model); got != tc.want {
			t.Errorf("isOrcaRouterDeepSeekModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}
