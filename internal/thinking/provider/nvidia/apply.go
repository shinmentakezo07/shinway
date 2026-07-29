// Package nvidia implements thinking configuration for NVIDIA NIM hosted models.
//
// NVIDIA NIM (https://build.nvidia.com, base URL https://integrate.api.nvidia.com/v1)
// speaks OpenAI chat completions but routes reasoning through a model-family
// specific chat_template_kwargs payload rather than the OpenAI reasoning_effort
// field. This package maps the canonical ThinkingConfig onto the per-model
// profiles defined in profiles.go, with a conservative default that simply
// toggles chat_template_kwargs.thinking for unknown model families.
package nvidia

import (
	"github.com/shinmentakezo07/shinway/v7/internal/registry"
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Applier implements thinking.ProviderApplier for NVIDIA NIM models.
type Applier struct{}

var _ thinking.ProviderApplier = (*Applier)(nil)

// NewApplier creates a new NVIDIA NIM thinking applier.
func NewApplier() *Applier {
	return &Applier{}
}

func init() {
	thinking.RegisterProvider("nvidia", NewApplier())
}

// Apply injects the appropriate chat_template_kwargs (and optional top-level
// reasoning_effort) into the request body for NVIDIA NIM.
func (a *Applier) Apply(body []byte, config thinking.ThinkingConfig, modelInfo *registry.ModelInfo) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		body = []byte(`{}`)
	}

	modelID := ""
	if modelInfo != nil {
		modelID = modelInfo.ID
	}
	if modelID == "" {
		// For user-defined / unknown NVIDIA models we still want to emit the
		// default thinking toggle. The pipeline hands us the requested model
		// via modelInfo whenever it knows it; otherwise the caller guarantees
		// ApplyThinking already resolved the suffix, so we can fall back to the
		// stored model name on the body.
		modelID = gjson.GetBytes(body, "model").String()
	}
	profile := Lookup(modelID)
	if profile.NoInject {
		return body, nil
	}

	canonical := canonicalLevel(config)

	// Inkling-style: always emit chat_template_kwargs {reasoning_effort: <level>}.
	if profile.Levels != nil {
		effort := profile.Levels[canonical]
		if effort == "" {
			effort = profile.Levels["auto"]
		}
		if effort == "" {
			return body, nil
		}
		result, errSet := sjson.SetBytes(body, kwargsThinkingPath+".reasoning_effort", effort)
		if errSet != nil {
			return body, nil
		}
		return result, nil
	}

	// MiniMax M3 style: chat_template_kwargs {thinking_mode: disabled|adaptive|enabled}.
	if profile.ModeLevels != nil {
		key := profile.ModeLevelsKey
		if key == "" {
			key = "thinking_mode"
		}
		mode := profile.ModeLevels[canonical]
		if mode == "" {
			mode = profile.ModeLevels["auto"]
		}
		if mode == "" {
			return body, nil
		}
		result, errSet := sjson.SetBytes(body, kwargsThinkingPath+"."+key, mode)
		if errSet != nil {
			return body, nil
		}
		return result, nil
	}

	isOff := canonical == string(thinking.LevelNone)

	// Pick the kwargs base (Enable or Disable).
	kwargs := profile.EnableKwargs
	if isOff {
		if profile.DisableKwargs != nil {
			kwargs = profile.DisableKwargs
		} else {
			kwargs = nil
		}
	}
	result := body
	if kwargs != nil {
		merged := mergeKwargs(result, kwargs)
		if merged == nil {
			return body, nil
		}
		result = merged
	}

	// Optionally embed reasoning_effort inside chat_template_kwargs (DeepSeek V4).
	if !isOff && profile.EffortInKwargs {
		effort := MapDeepSeekLevel(canonical)
		if effort == "" {
			return result, nil
		}
		updated, errSet := sjson.SetBytes(result, kwargsThinkingPath+".reasoning_effort", effort)
		if errSet == nil {
			result = updated
		}
	}

	// Optionally emit top-level reasoning_effort (Kimi).
	if !isOff && profile.SendTopLevelEffort {
		effort := MapLevel(canonical)
		if effort == string(thinking.LevelNone) {
			effort = ""
		}
		if effort != "" && effort != string(thinking.LevelAuto) {
			updated, errSet := sjson.SetBytes(result, "reasoning_effort", effort)
			if errSet == nil {
				result = updated
			}
		}
	}
	return result, nil
}

// canonicalLevel collapses the ThinkingConfig mode/level into a string label
// that the profile maps understand.
func canonicalLevel(config thinking.ThinkingConfig) string {
	switch config.Mode {
	case thinking.ModeNone:
		return string(thinking.LevelNone)
	case thinking.ModeAuto:
		return string(thinking.LevelAuto)
	case thinking.ModeLevel:
		if config.Level == "" {
			return string(thinking.LevelMedium)
		}
		return string(config.Level)
	case thinking.ModeBudget:
		if config.Budget == 0 {
			return string(thinking.LevelNone)
		}
		if config.Budget < 0 {
			return string(thinking.LevelAuto)
		}
		if level, ok := thinking.ConvertBudgetToLevel(config.Budget); ok {
			return level
		}
		return string(thinking.LevelMedium)
	default:
		return string(thinking.LevelAuto)
	}
}

// mergeKwargs deep-merges src into dst[chat_template_kwargs].
// Returns nil when merge fails (defensive: caller should fall back).
func mergeKwargs(dst []byte, src Kwargs) []byte {
	result := dst
	for key, value := range src {
		updated, errSet := sjson.SetBytes(result, kwargsThinkingPath+"."+key, value)
		if errSet != nil {
			return nil
		}
		result = updated
	}
	return result
}
