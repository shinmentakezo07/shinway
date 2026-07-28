package api

import (
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/config"
)

func TestEffectiveSDKConfigCopiesCodexOptimizeMultiAgentV2(t *testing.T) {
	cfg := &config.Config{Codex: config.CodexConfig{OptimizeMultiAgentV2: true}}

	sdkCfg := effectiveSDKConfig(cfg)
	if sdkCfg == nil || !sdkCfg.CodexOptimizeMultiAgentV2 {
		t.Fatalf("CodexOptimizeMultiAgentV2 = false, want true")
	}
}
