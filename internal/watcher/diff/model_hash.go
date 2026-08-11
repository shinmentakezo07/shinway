package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/internal/registry"
)

// ComputeOpenAICompatModelsHash returns a stable hash for OpenAI-compat models.
// Used to detect model list changes during hot reload.
func ComputeOpenAICompatModelsHash(models []config.OpenAICompatibilityModel) string {
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(modelHashKey(name, alias, model.DisplayName, model.ForceMapping, model.IsCompat, model.MaxContextLength, model.Thinking, fmt.Sprintf("image=%t", model.Image)))
		}
	})
	return hashJoined(keys)
}

// ComputeVertexCompatModelsHash returns a stable hash for Vertex-compatible models.
func ComputeVertexCompatModelsHash(models []config.VertexCompatModel) string {
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(modelHashKey(name, alias, model.DisplayName, model.ForceMapping, model.IsCompat, model.MaxContextLength, model.Thinking))
		}
	})
	return hashJoined(keys)
}

// ComputeClaudeModelsHash returns a stable hash for Claude model aliases.
func ComputeClaudeModelsHash(models []config.ClaudeModel) string {
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(modelHashKey(name, alias, model.DisplayName, model.ForceMapping, model.IsCompat, model.MaxContextLength, model.Thinking))
		}
	})
	return hashJoined(keys)
}

// ComputeCodexModelsHash returns a stable hash for Codex model aliases.
func ComputeCodexModelsHash(models []config.CodexModel) string {
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(modelHashKey(name, alias, model.DisplayName, model.ForceMapping, model.IsCompat, model.MaxContextLength, model.Thinking))
		}
	})
	return hashJoined(keys)
}

// ComputeGeminiModelsHash returns a stable hash for Gemini model aliases.
func ComputeGeminiModelsHash(models []config.GeminiModel) string {
	keys := normalizeModelPairs(func(out func(key string)) {
		for _, model := range models {
			name := strings.TrimSpace(model.Name)
			alias := strings.TrimSpace(model.Alias)
			if name == "" && alias == "" {
				continue
			}
			out(modelHashKey(name, alias, model.DisplayName, model.ForceMapping, model.IsCompat, model.MaxContextLength, model.Thinking))
		}
	})
	return hashJoined(keys)
}

func modelHashKey(name, alias, displayName string, forceMapping, isCompat bool, maxContextLength int, thinking *registry.ThinkingSupport, suffix ...string) string {
	key := strings.ToLower(strings.TrimSpace(name)) + "|" + strings.ToLower(strings.TrimSpace(alias)) + "|" + strings.TrimSpace(displayName) + "|" + fmt.Sprintf("force-mapping=%t|is-compat=%t|max-context-length=%d", forceMapping, isCompat, maxContextLength) + thinkingHashSuffix(thinking)
	if len(suffix) > 0 && suffix[0] != "" {
		key += "|" + suffix[0]
	}
	return key
}

func thinkingHashSuffix(support *registry.ThinkingSupport) string {
	data, _ := json.Marshal(support)
	return "|thinking=" + string(data)
}

// ComputeExcludedModelsHash returns a normalized hash for excluded model lists.
func ComputeExcludedModelsHash(excluded []string) string {
	if len(excluded) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(excluded))
	for _, entry := range excluded {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			normalized = append(normalized, strings.ToLower(trimmed))
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	sort.Strings(normalized)
	data, _ := json.Marshal(normalized)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func modelCapabilityHashSuffix(maxContextLength int, isCompat bool, support *registry.ThinkingSupport) string {
	data, _ := json.Marshal(support)
	return fmt.Sprintf("|max-context-length=%d|is-compat=%t|thinking=%s", maxContextLength, isCompat, data)
}

func normalizeModelPairs(collect func(out func(key string))) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	collect(func(key string) {
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	})
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return keys
}

func hashJoined(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join(keys, "\n")))
	return hex.EncodeToString(sum[:])
}
