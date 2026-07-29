package nvidia

import (
	"strings"
)

// Kwargs is a raw JSON-value map merged under chat_template_kwargs.
type Kwargs map[string]any

// Profile describes how a specific NVIDIA NIM model family wants its thinking
// (reasoning) signal delivered.
//
// Verified against NVIDIA's deploy service behavior (see build.nvidia.com docs,
// the pi-nvidia-nim project notes, and the GitHub issue trails for OpenClaw /
// anomalyco opencode): every NIM reasoning model talks OpenAI chat completions
// but requires a model-family specific chat_template_kwargs payload. Top-level
// reasoning_effort is ignored by most models; a couple of them support it either
// along-side the kwargs (Kimi) or inside the kwargs (DeepSeek V4).
type Profile struct {
	// EnableKwargs is merged into chat_template_kwargs when thinking is enabled.
	EnableKwargs Kwargs
	// DisableKwargs is merged into chat_template_kwargs when thinking is explicitly off.
	DisableKwargs Kwargs
	// EffortInKwargs merges the mapped reasoning_effort into chat_template_kwargs
	// together with EnableKwargs (DeepSeek V4 expects this).
	EffortInKwargs bool
	// SendTopLevelEffort also forwards the mapped reasoning_effort as a top-level
	// body field (Kimi accepts it in addition to the kwargs).
	SendTopLevelEffort bool
	// Levels describes the discrete reasoning levels the model understands when
	// efforts are surfaced through chat_template_kwargs (e.g. Inkling's presets).
	Levels map[string]string
	// ModeLevels describes mode-based thinking ON/OFF switches for models like MiniMax M3.
	ModeLevels map[string]string
	// ModeLevelsKey is the kwargs key used for ModeLevels injection.
	ModeLevelsKey string
	// NoInject disables every automatic injection for this model.
	NoInject bool
}

var (
	// kwargsThinkingPath is the JSON path where thinking kwargs are merged.
	kwargsThinkingPath = "chat_template_kwargs"
)

// defaultProfile kicks in when the model ID has no profile match. All NIM
// reasoning models observed so far accept {thinking: true|false}; models that
// don't reason simply ignore the kwargs.
var defaultProfile = Profile{
	EnableKwargs:  Kwargs{"thinking": true},
	DisableKwargs: Kwargs{"thinking": false},
}

// Profiles maps normalized model IDs to their family-specific thinking profile.
var Profiles = map[string]Profile{
	// ---------- DeepSeek ----------
	"deepseek-ai/deepseek-v4-flash": {
		EnableKwargs:   Kwargs{"thinking": true},
		DisableKwargs:  Kwargs{"thinking": false},
		EffortInKwargs: true,
	},
	"deepseek-ai/deepseek-v4-pro": {
		EnableKwargs:   Kwargs{"thinking": true},
		DisableKwargs:  Kwargs{"thinking": false},
		EffortInKwargs: true,
	},
	"deepseek-ai/deepseek-v3.1": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-v3.1-terminus": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-v3.2": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-r1-distill-llama-8b": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-r1-distill-qwen-7b": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-r1-distill-qwen-14b": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"deepseek-ai/deepseek-r1-distill-qwen-32b": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},

	// ---------- Z-AI / GLM ----------
	"z-ai/glm5": {
		EnableKwargs:  Kwargs{"enable_thinking": true, "clear_thinking": false},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},
	"z-ai/glm4.7": {
		EnableKwargs:  Kwargs{"enable_thinking": true, "clear_thinking": false},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},

	// ---------- Moonshot / Kimi ----------
	"moonshotai/kimi-k2.6": {
		EnableKwargs:       Kwargs{"thinking": true},
		DisableKwargs:      Kwargs{"thinking": false},
		SendTopLevelEffort: true,
	},
	"moonshotai/kimi-k2-thinking": {
		EnableKwargs:       Kwargs{"thinking": true},
		DisableKwargs:      Kwargs{"thinking": false},
		SendTopLevelEffort: true,
	},

	// ---------- Qwen ----------
	"qwen/qwen3-235b-a22b": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},
	"qwen/qwen3-coder-480b-a35b-instruct": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},
	"qwen/qwen3-next-80b-a3b-thinking": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},
	"qwen/qwq-32b": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},

	// ---------- Phi ----------
	"microsoft/phi-4-mini-flash-reasoning": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},

	// ---------- NVIDIA Nemotron ----------
	"nvidia/llama-3.1-nemotron-ultra-253b-v1": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"nvidia/llama-3.3-nemotron-super-49b-v1": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},
	"nvidia/llama-3.3-nemotron-super-49b-v1.5": {
		EnableKwargs:  Kwargs{"thinking": true},
		DisableKwargs: Kwargs{"thinking": false},
	},

	// ---------- Mistral ----------
	"mistralai/magistral-small-2506": {
		EnableKwargs:  Kwargs{"enable_thinking": true},
		DisableKwargs: Kwargs{"enable_thinking": false},
	},

	// ---------- Special chat-template modes ----------
	"thinkingmachines/inkling": {
		Levels: map[string]string{
			"off":     "none",
			"none":    "none",
			"minimal": "minimal",
			"low":     "low",
			"medium":  "medium",
			"high":    "high",
			"xhigh":   "max",
			"max":     "max",
			"auto":    "medium",
		},
	},
	"minimaxai/minimax-m3": {
		ModeLevels: map[string]string{
			"off":     "disabled",
			"none":    "disabled",
			"minimal": "adaptive",
			"low":     "adaptive",
			"medium":  "adaptive",
			"high":    "enabled",
			"xhigh":   "enabled",
			"max":     "enabled",
			"auto":    "adaptive",
		},
		ModeLevelsKey: "thinking_mode",
	},
}

// Lookup returns the thinking profile for a given NVIDIA NIM model ID,
// falling back to the default thinking toggle when no exact or prefix
// match is found.
func Lookup(modelID string) Profile {
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	if modelID == "" {
		return defaultProfile
	}
	if profile, ok := Profiles[modelID]; ok {
		return profile
	}
	// Longest-prefix match so future variants (e.g. "qwen/qwen3-coder-480b-a35b-instruct-v2")
	// inherit their family's profile.
	bestLen := -1
	var best Profile
	for id, profile := range Profiles {
		if !strings.HasPrefix(modelID, id) {
			// Also try "id/" boundary families like "deepseek-ai/deepseek-r1-distill-qwen-32b-instruct".
			if !strings.HasPrefix(modelID, id+"/") && !strings.HasPrefix(modelID, id+"-") {
				continue
			}
		}
		if len(id) > bestLen {
			bestLen = len(id)
			best = profile
		}
	}
	if bestLen > 0 {
		return best
	}
	return defaultProfile
}

// MapLevel converts a canonical level ("none"|"minimal"|"low"|"medium"|"high"|"xhigh"|"max"|"auto")
// into the closest level this NIM model accepts. The top-level reasoning_effort
// values used by NIM (when a model even reads them) are restricted to
// {"low", "medium", "high"}; xhigh/max are widened to "high".
func MapLevel(canonical string) string {
	switch canonical {
	case "minimal":
		return "low"
	case "xhigh", "max":
		return "high"
	case "none", "low", "medium", "high", "auto":
		return canonical
	default:
		return "high"
	}
}

// MapDeepSeekLevel maps canonical levels into the only two values DeepSeek V4
// accepts inside chat_template_kwargs.reasoning_effort.
func MapDeepSeekLevel(canonical string) string {
	switch canonical {
	case "xhigh", "max":
		return "max"
	default:
		return "high"
	}
}
