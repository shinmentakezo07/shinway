package helps

import (
	"bytes"
	"context"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	sdktranslator "github.com/shinmentakezo07/shinway/v7/sdk/translator"
)

func TestTranslateRequestWithAPIKeyModelCompatibility(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	claudeBody := []byte(`{"model":"claude-sonnet-4-5","max_tokens":1000,"messages":[{"role":"user","content":"hi"}]}`)
	codexBody := []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	t.Run("non-compat delegates to standard translation", func(t *testing.T) {
		from, to := sdktranslator.FormatClaude, sdktranslator.FormatCodex
		out := TranslateRequestWithAPIKeyModelCompatibility(ctx, nil, cfg, from, to, "claude-sonnet-4-5", claudeBody, false, false)
		want := TranslateRequestWithCodexMultiAgentV2(ctx, nil, cfg, from, to, "claude-sonnet-4-5", claudeBody, false)
		if string(out) != string(want) {
			t.Fatalf("non-compat output diverges from standard translation:\n got: %s\nwant: %s", out, want)
		}
	})

	t.Run("compat preserves assistant thinking from claude to codex", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":1000,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning","signature":"EgI="},{"type":"text","text":"ok"}]}]}`)
		out := TranslateRequestWithAPIKeyModelCompatibility(ctx, nil, cfg, sdktranslator.FormatClaude, sdktranslator.FormatCodex, "claude-sonnet-4-5", body, false, true)
		if !contains(out, []byte("reasoning")) {
			t.Fatalf("compat translation dropped assistant thinking text:\n%s", out)
		}
	})

	t.Run("compat openai chat to claude", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hi"}]}`)
		out := TranslateRequestWithAPIKeyModelCompatibility(ctx, nil, cfg, sdktranslator.FormatOpenAI, sdktranslator.FormatClaude, "gpt-5", body, false, true)
		if !contains(out, []byte(`"role":"user"`)) && !contains(out, []byte(`"role": "user"`)) {
			t.Fatalf("compat openai->claude produced unexpected output:\n%s", out)
		}
	})

	t.Run("compat codex responses to claude rewrites multi-agent input", func(t *testing.T) {
		out := TranslateRequestWithAPIKeyModelCompatibility(ctx, nil, cfg, sdktranslator.FormatOpenAIResponse, sdktranslator.FormatClaude, "gpt-5", codexBody, false, true)
		if len(out) == 0 {
			t.Fatal("compat responses->claude returned empty output")
		}
	})
}

func contains(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
