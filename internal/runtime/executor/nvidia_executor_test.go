package executor

import (
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/tidwall/gjson"
)

func TestNVIDIAExecutorHook_ReplacesDeveloperRole(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"messages":[{"role":"developer","content":"do x"},{"role":"user","content":"hi"}]}`)
	out := exec.translateHook(body, "acme/unknown")
	if gjson.GetBytes(out, "messages.0.role").String() != "system" {
		t.Fatalf("expected developer -> system, got %s", out)
	}
	if gjson.GetBytes(out, "messages.1.role").String() != "user" {
		t.Fatalf("user role must be preserved, got %s", out)
	}
}

func TestNVIDIAExecutorHook_FlattensTextContent(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}`)
	out := exec.translateHook(body, "acme/unknown")
	got := gjson.GetBytes(out, "messages.0.content")
	if got.String() != "a\nb" {
		t.Fatalf("expected joined string, got %s", got.String())
	}
}

func TestNVIDIAExecutorHook_PreservesNonTextContent(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`)
	out := exec.translateHook(body, "acme/unknown")
	got := gjson.GetBytes(out, "messages.0.content")
	if got.Get("0.type").String() != "image_url" {
		t.Fatalf("expected non-text content untouched, got %s", got.String())
	}
}

func TestNVIDIAExecutorHook_RenamesMaxCompletionTokens(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"max_completion_tokens":256}`)
	out := exec.translateHook(body, "acme/unknown")
	if gjson.GetBytes(out, "max_completion_tokens").Exists() {
		t.Fatalf("max_completion_tokens must be removed, got %s", out)
	}
	if gjson.GetBytes(out, "max_tokens").Int() != 256 {
		t.Fatalf("expected max_tokens=256, got %s", out)
	}
}

func TestNVIDIAExecutorHook_StripsReasoningEffort(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"reasoning_effort":"high"}`)
	out := exec.translateHook(body, "deepseek-ai/deepseek-v4-flash")
	if gjson.GetBytes(out, "reasoning_effort").Exists() {
		t.Fatalf("reasoning_effort should be stripped for DeepSeek, got %s", out)
	}
}

func TestNVIDIAExecutorHook_KeepsReasoningEffortForKimi(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"reasoning_effort":"high"}`)
	out := exec.translateHook(body, "moonshotai/kimi-k2.6")
	if gjson.GetBytes(out, "reasoning_effort").String() != "high" {
		t.Fatalf("Kimi profile keeps top-level reasoning_effort, got %s", out)
	}
}

func TestNVIDIAExecutorHook_DefaultInjectsThinkingForUnknownModel(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	out := exec.translateHook([]byte(`{"messages":[]}`), "acme/unknown")
	if !gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("expected default inject thinking=true, got %s", out)
	}
}

func TestNVIDIAExecutorHook_DefaultDisabledViaConfig(t *testing.T) {
	off := false
	exec := NewNVIDIAExecutor(&config.Config{NVIDIA: config.NVIDIAConfig{DefaultThinking: &off}})
	out := exec.translateHook([]byte(`{"messages":[]}`), "acme/unknown")
	if gjson.GetBytes(out, "chat_template_kwargs").Exists() {
		t.Fatalf("default-thinking=false should suppress inject, got %s", out)
	}
}

func TestNVIDIAExecutorHook_SkipsDefaultWhenBodyHasKwargs(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	body := []byte(`{"chat_template_kwargs":{"thinking":false}}`)
	out := exec.translateHook(body, "acme/unknown")
	if gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("must not override caller-provided kwargs, got %s", out)
	}
}

func TestNVIDIAExecutorHook_AppliesSuffixThinkingAndStripsModelSuffix(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	out := exec.translateHook([]byte(`{"messages":[],"model":"acme/unknown(high)"}`), "acme/unknown(high)")
	outModel := exec.effectiveModel("acme/unknown(high)")
	// Suffix handling happens through thinking.ApplyThinking (default profile:
	// chat_template_kwargs.thinking = true) not through the step-5 default
	// inject, and the model suffix must be stripped for the inner executor.
	if !gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("suffix should inject thinking=true via applier, got %s", out)
	}
	if outModel != "acme/unknown" {
		t.Fatalf("expected model suffix stripped, got %q", outModel)
	}
}

func TestNVIDIAExecutorHook_InjectsDefaultForDeepSeekV4(t *testing.T) {
	exec := NewNVIDIAExecutor(&config.Config{})
	out := exec.translateHook([]byte(`{"messages":[]}`), "deepseek-ai/deepseek-v4-flash")
	if !gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("expected thinking=true, got %s", out)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.reasoning_effort").String() != "high" {
		t.Fatalf("expected reasoning_effort=high baked in kwargs, got %s", out)
	}
}
