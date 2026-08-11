package auth

import (
	"testing"

	internalconfig "github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/internal/registry"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

func TestAttachResolvedAPIKeyModelInfoUsesSelectedCredential(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{ClaudeKey: []internalconfig.ClaudeKey{
		{APIKey: "key-high", Prefix: "tenant", Models: []internalconfig.ClaudeModel{{Name: "shared-upstream", Alias: "public-model", Thinking: &registry.ThinkingSupport{Levels: []string{"high"}}}}},
		{APIKey: "key-max", Prefix: "tenant", Models: []internalconfig.ClaudeModel{{Name: "shared-upstream", Alias: "public-model", Thinking: &registry.ThinkingSupport{Levels: []string{"max"}}}}},
	}})

	authHigh := configuredCapabilityTestAuth("auth-high", "key-high")
	authMax := configuredCapabilityTestAuth("auth-max", "key-max")
	registerCapabilityTestAuth(t, manager, authHigh)
	registerCapabilityTestAuth(t, manager, authMax)

	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, authHigh, "tenant/public-model", "shared-upstream"), "high")
	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, authMax, "tenant/public-model", "shared-upstream"), "max")
}

func TestAttachResolvedAPIKeyModelInfoPrefersExactConfiguredSuffix(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	auth := configuredCapabilityTestAuth("auth-suffix", "key-suffix")
	manager.SetConfig(&internalconfig.Config{ClaudeKey: []internalconfig.ClaudeKey{{
		APIKey: "key-suffix", Prefix: "tenant",
		Models: []internalconfig.ClaudeModel{
			{Name: "shared-upstream(high)", Alias: "public-high", Thinking: &registry.ThinkingSupport{Levels: []string{"high"}}},
			{Name: "shared-upstream(low)", Alias: "public-low", Thinking: &registry.ThinkingSupport{Levels: []string{"low"}}},
			{Name: "alias-upstream", Alias: "public(high)", Thinking: &registry.ThinkingSupport{Levels: []string{"high"}}},
			{Name: "alias-upstream", Alias: "public(low)", Thinking: &registry.ThinkingSupport{Levels: []string{"low"}}},
		},
	}}})
	registerCapabilityTestAuth(t, manager, auth)

	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/public-low", "shared-upstream(low)"), "low")

	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/shared-upstream(low)", "shared-upstream(low)"), "low")
	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/public(low)", "alias-upstream(low)"), "low")
}

func TestAPIKeyModelRoutingSnapshotsConfigAcrossReload(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	auth := configuredCapabilityTestAuth("auth-reload", "key-reload")
	buildConfig := func(level string) *internalconfig.Config {
		return &internalconfig.Config{ClaudeKey: []internalconfig.ClaudeKey{{
			APIKey: "key-reload", Prefix: "tenant",
			Models: []internalconfig.ClaudeModel{{Name: "shared-upstream", Alias: "public", Thinking: &registry.ThinkingSupport{Levels: []string{level}}}},
		}}}
	}
	manager.SetConfig(buildConfig("high"))
	registerCapabilityTestAuth(t, manager, auth)
	models, _, _, routing := manager.executionModelCandidatesWithAliasWithRouting(auth, "tenant/public")
	if len(models) != 1 || models[0] != "shared-upstream" {
		t.Fatalf("execution models = %v, want [shared-upstream]", models)
	}

	manager.SetConfig(buildConfig("max"))
	assertResolvedThinkingLevels(t, attachResolvedAPIKeyModelInfo(routing, shinwayexecutor.Request{}, auth, "tenant/public", models[0]), "high")
	assertResolvedThinkingLevels(t, manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/public", models[0]), "max")
}

func TestAttachResolvedAPIKeyModelInfoSupportsKeylessOpenAICompatibility(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{OpenAICompatibility: []internalconfig.OpenAICompatibility{{
		Name: "keyless", Prefix: "tenant", BaseURL: "https://example.com/v1",
		Models: []internalconfig.OpenAICompatibilityModel{{Name: "shared-upstream", Alias: "public-model", ForceMapping: true, IsCompat: true, Thinking: &registry.ThinkingSupport{Levels: []string{"high"}}}},
	}}})
	auth := &Auth{ID: "auth-keyless", Provider: "openai-compatibility:keyless", Prefix: "tenant", Attributes: map[string]string{
		AttributeAuthKind: AuthKindAPIKey, AttributeSource: "config:keyless[0]", "compat_name": "keyless", "provider_key": "openai-compatibility:keyless",
	}}
	registerCapabilityTestAuth(t, manager, auth)

	models, _, aliasResult := manager.executionModelCandidatesWithAlias(auth, "tenant/public-model")
	if len(models) != 1 || models[0] != "shared-upstream" || !aliasResult.ForceMapping {
		t.Fatalf("keyless execution result = %v, %+v", models, aliasResult)
	}
	req := manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/public-model", models[0])
	assertResolvedThinkingLevels(t, req, "high")
	info, ok := ResolvedAPIKeyModelInfo(req)
	if !ok || !info.IsCompat {
		t.Fatalf("resolved IsCompat = (%+v, %t), want true", info, ok)
	}
}

func TestAttachResolvedAPIKeyModelInfoBindsUnknownConfiguredCapability(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	auth := configuredCapabilityTestAuth("auth-fallback", "key-fallback")
	manager.SetConfig(&internalconfig.Config{ClaudeKey: []internalconfig.ClaudeKey{{
		APIKey: "key-fallback", Prefix: "tenant", Models: []internalconfig.ClaudeModel{{Name: "unknown-upstream", Alias: "unknown-public"}},
	}}})
	registerCapabilityTestAuth(t, manager, auth)

	info, ok := ResolvedAPIKeyModelInfo(manager.attachResolvedAPIKeyModelInfo(shinwayexecutor.Request{}, auth, "tenant/unknown-public", "unknown-upstream"))
	if !ok || info == nil || info.UserDefined || info.Thinking != nil {
		t.Fatalf("resolved model = (%+v, %t), want authoritative empty capability", info, ok)
	}
}

func registerCapabilityTestAuth(t *testing.T, manager *Manager, auth *Auth) {
	t.Helper()
	if registered, err := manager.Register(t.Context(), auth); err != nil || registered == nil {
		t.Fatalf("Register() = (%+v, %v), want registered auth", registered, err)
	}
}

func configuredCapabilityTestAuth(id, apiKey string) *Auth {
	return &Auth{ID: id, Provider: "claude", Prefix: "tenant", Attributes: map[string]string{
		AttributeAuthKind: AuthKindAPIKey, AttributeAPIKey: apiKey, AttributeSource: "config:claude[0]",
	}}
}

func assertResolvedThinkingLevels(t *testing.T, req shinwayexecutor.Request, want ...string) {
	t.Helper()
	info, ok := ResolvedAPIKeyModelInfo(req)
	if !ok || info == nil || info.Thinking == nil {
		t.Fatalf("ResolvedAPIKeyModelInfo() = (%+v, %t), want thinking levels %v", info, ok, want)
	}
	if len(info.Thinking.Levels) != len(want) {
		t.Fatalf("thinking levels = %v, want %v", info.Thinking.Levels, want)
	}
	for i := range want {
		if info.Thinking.Levels[i] != want[i] {
			t.Fatalf("thinking levels = %v, want %v", info.Thinking.Levels, want)
		}
	}
}
