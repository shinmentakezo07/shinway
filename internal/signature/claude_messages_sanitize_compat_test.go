package signature

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSanitizeClaudeMessagesForClaudeUpstreamPreservesEmptyThinkingInCompatMode(t *testing.T) {
	input := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":""}]}]}`)

	withoutCompat, _ := SanitizeClaudeMessagesForClaudeUpstream(input, "deepseek-v4")
	if gjson.GetBytes(withoutCompat, "messages.0.content.#").Int() != 0 {
		t.Fatalf("default sanitizer preserved empty thinking: %s", withoutCompat)
	}

	withCompat, _ := SanitizeClaudeMessagesForClaudeUpstream(input, "deepseek-v4", true)
	part := gjson.GetBytes(withCompat, "messages.0.content.0")
	if part.Get("type").String() != "thinking" || part.Get("signature").String() != "" {
		t.Fatalf("compat sanitizer dropped empty thinking: %s", withCompat)
	}
}

func TestSanitizeClaudeMessagesForClaudeUpstreamPreservesOpaqueThinkingSignatureInCompatMode(t *testing.T) {
	input := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"reason","signature":"opaque-deepseek-id"}]}]}`)

	withoutCompat, _ := SanitizeClaudeMessagesForClaudeUpstream(input, "deepseek-v4")
	if gjson.GetBytes(withoutCompat, "messages.0.content.0.signature").String() != "" {
		t.Fatalf("default sanitizer preserved opaque signature: %s", withoutCompat)
	}

	withCompat, _ := SanitizeClaudeMessagesForClaudeUpstream(input, "deepseek-v4", true)
	part := gjson.GetBytes(withCompat, "messages.0.content.0")
	if part.Get("type").String() != "thinking" || part.Get("signature").String() != "opaque-deepseek-id" {
		t.Fatalf("compat sanitizer dropped opaque signature: %s", withCompat)
	}
}

func TestSanitizeClaudeMessagesForClaudeUpstreamCompatKeepsToolUsePayload(t *testing.T) {
	input := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"provider reasoning","signature":"EgI="},{"type":"tool_use","id":"toolu_1","name":"Read","input":{"path":"README.md"}}]}]}`)

	withCompat, _ := SanitizeClaudeMessagesForClaudeUpstream(input, "claude-synthetic-4772", true)
	content := gjson.GetBytes(withCompat, "messages.0.content")
	if content.Get("#").Int() != 2 {
		t.Fatalf("compat sanitizer changed content block count: %s", withCompat)
	}
	if content.Get("0.type").String() != "thinking" || content.Get("0.signature").String() != "EgI=" {
		t.Fatalf("compat sanitizer altered restored thinking block: %s", withCompat)
	}
	if content.Get("1.type").String() != "tool_use" || content.Get("1.id").String() != "toolu_1" {
		t.Fatalf("compat sanitizer altered tool_use block: %s", withCompat)
	}
}
