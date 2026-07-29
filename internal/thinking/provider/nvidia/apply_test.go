package nvidia

import (
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/registry"
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	"github.com/tidwall/gjson"
)

func TestApply_DefaultProfile_UnknownModel_InjectsThinkingTrue(t *testing.T) {
	applier := NewApplier()
	body := []byte(`{"model":"acme/brand-new-model","messages":[{"role":"user","content":"hi"}]}`)
	out, err := applier.Apply(body, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelMedium}, nil)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	got := gjson.GetBytes(out, "chat_template_kwargs.thinking")
	if !got.Exists() || !got.Bool() {
		t.Fatalf("expected chat_template_kwargs.thinking=true, got body=%s", out)
	}
	if gjson.GetBytes(out, "reasoning_effort").Exists() {
		t.Fatalf("should not emit top-level reasoning_effort for unknown model, got %s", out)
	}
}

func TestApply_UnknownModel_DisableThinking(t *testing.T) {
	applier := NewApplier()
	body := []byte(`{"model":"acme/foo"}`)
	out, err := applier.Apply(body, thinking.ThinkingConfig{Mode: thinking.ModeNone}, nil)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	got := gjson.GetBytes(out, "chat_template_kwargs.thinking")
	if !got.Exists() || got.Bool() {
		t.Fatalf("expected chat_template_kwargs.thinking=false, got %s", out)
	}
}

func TestApply_DeepSeekV4(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "deepseek-ai/deepseek-v4-flash"}
	body := []byte(`{"model":"deepseek-ai/deepseek-v4-flash"}`)
	out, err := applier.Apply(body, thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("expected thinking=true, got %s", out)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.reasoning_effort").String() != "high" {
		t.Fatalf("expected reasoning_effort=high inside kwargs, got %s", out)
	}
	if gjson.GetBytes(out, "reasoning_effort").Exists() {
		t.Fatalf("must not emit top-level reasoning_effort for deepseek, got %s", out)
	}
}

func TestApply_DeepSeekV4_XHighMapsToMax(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "deepseek-ai/deepseek-v4-pro"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.reasoning_effort").String() != "max" {
		t.Fatalf("expected reasoning_effort=max inside kwargs, got %s", out)
	}
}

func TestApply_GLM5(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "z-ai/glm5"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !gjson.GetBytes(out, "chat_template_kwargs.enable_thinking").Bool() {
		t.Fatalf("expected enable_thinking=true, got %s", out)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.clear_thinking").Bool() {
		t.Fatalf("expected clear_thinking=false, got %s", out)
	}
}

func TestApply_GLM5_Off(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "z-ai/glm5"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeNone}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.enable_thinking").Bool() {
		t.Fatalf("expected enable_thinking=false, got %s", out)
	}
}

func TestApply_Kimi(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "moonshotai/kimi-k2.6"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !gjson.GetBytes(out, "chat_template_kwargs.thinking").Bool() {
		t.Fatalf("expected kwargs thinking=true, got %s", out)
	}
	if gjson.GetBytes(out, "reasoning_effort").String() != "high" {
		t.Fatalf("expected top-level reasoning_effort=high for Kimi, got %s", out)
	}
}

func TestApply_Qwen3Coder(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "qwen/qwen3-coder-480b-a35b-instruct"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !gjson.GetBytes(out, "chat_template_kwargs.enable_thinking").Bool() {
		t.Fatalf("expected enable_thinking=true, got %s", out)
	}
}

func TestApply_Inkling(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "thinkingmachines/inkling"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelXHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.reasoning_effort").String() != "max" {
		t.Fatalf("expected reasoning_effort=max in kwargs for Inkling, got %s", out)
	}
}

func TestApply_MiniMaxM3(t *testing.T) {
	applier := NewApplier()
	info := &registry.ModelInfo{ID: "minimaxai/minimax-m3"}
	out, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeLevel, Level: thinking.LevelHigh}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if gjson.GetBytes(out, "chat_template_kwargs.thinking_mode").String() != "enabled" {
		t.Fatalf("expected thinking_mode=enabled, got %s", out)
	}
	outOff, err := applier.Apply([]byte(`{}`), thinking.ThinkingConfig{Mode: thinking.ModeNone}, info)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if gjson.GetBytes(outOff, "chat_template_kwargs.thinking_mode").String() != "disabled" {
		t.Fatalf("expected thinking_mode=disabled, got %s", outOff)
	}
}

func TestLookup_LongestPrefix(t *testing.T) {
	profile := Lookup("deepseek-ai/deepseek-v4-flash-weekly-rc")
	if profile.EnableKwargs == nil {
		t.Fatalf("expected prefix-match on deepseek-v4-flash, got %+v", profile)
	}
	if !profile.EffortInKwargs {
		t.Fatalf("expected EffortInKwargs from deepseek profile, got %+v", profile)
	}
}

func TestMapLevel(t *testing.T) {
	cases := map[string]string{
		"minimal": "low",
		"xhigh":   "high",
		"max":     "high",
		"low":     "low",
		"medium":  "medium",
		"high":    "high",
		"none":    "none",
		"weird":   "high",
	}
	for in, want := range cases {
		if got := MapLevel(in); got != want {
			t.Errorf("MapLevel(%q) = %q, want %q", in, got, want)
		}
	}
}
