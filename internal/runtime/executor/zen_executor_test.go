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
