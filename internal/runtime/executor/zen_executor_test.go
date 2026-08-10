package executor

import (
	"context"
	"net/http"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
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
