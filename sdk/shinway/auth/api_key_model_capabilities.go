package auth

import (
	"maps"
	"strings"

	internalconfig "github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/internal/registry"
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

const resolvedAPIKeyModelInfoMetadataKey = "shinway.resolved_api_key_model_info"

type apiKeyModelCapabilityRoute struct {
	upstreamModel string
	modelInfo     *registry.ModelInfo
}

type apiKeyModelCapabilityTable map[string]map[string][]apiKeyModelCapabilityRoute

// apiKeyModelRoutingSnapshot is immutable once published. Keeping all model
// routing data in one value prevents a hot reload from resolving an alias from
// one configuration generation and capabilities from another.
type apiKeyModelRoutingSnapshot struct {
	config       *internalconfig.Config
	aliases      apiKeyModelAliasTable
	capabilities apiKeyModelCapabilityTable
}

func (m *Manager) loadAPIKeyModelRouting() *apiKeyModelRoutingSnapshot {
	if m == nil {
		return &apiKeyModelRoutingSnapshot{config: &internalconfig.Config{}}
	}
	snapshot, _ := m.apiKeyModelRouting.Load().(*apiKeyModelRoutingSnapshot)
	if snapshot == nil {
		return &apiKeyModelRoutingSnapshot{config: &internalconfig.Config{}}
	}
	return snapshot
}

// ResolvedAPIKeyModelInfo returns the exact configured model capability snapshot
// bound to one API-key execution attempt.
func ResolvedAPIKeyModelInfo(req shinwayexecutor.Request) (*registry.ModelInfo, bool) {
	modelInfo, ok := req.Metadata[resolvedAPIKeyModelInfoMetadataKey].(*registry.ModelInfo)
	return modelInfo, ok && modelInfo != nil
}

// CodexAPIKeyModelIsCompat reports whether the selected codex-api-key model has
// is-compat enabled. When true and codex.optimize-multi-agent-v2 is also true,
// Codex MultiAgentV2 agent_message items are converted into portable Responses
// message/user input for third-party Responses-compatible endpoints.
func CodexAPIKeyModelIsCompat(cfg *internalconfig.Config, auth *Auth, model string) bool {
	if cfg == nil || auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
		return false
	}
	entry := resolveCodexAPIKeyConfig(cfg, auth)
	if entry == nil || len(entry.Models) == 0 {
		return false
	}
	requested := strings.TrimSpace(model)
	if requested == "" {
		return false
	}
	baseModel := strings.TrimSpace(thinking.ParseSuffix(requested).ModelName)
	if baseModel == "" {
		baseModel = requested
	}
	for i := range entry.Models {
		name := strings.TrimSpace(entry.Models[i].Name)
		alias := strings.TrimSpace(entry.Models[i].Alias)
		if name == "" {
			name = alias
		}
		if alias == "" {
			alias = name
		}
		if name == "" {
			continue
		}
		if strings.EqualFold(name, requested) || strings.EqualFold(name, baseModel) ||
			strings.EqualFold(alias, requested) || strings.EqualFold(alias, baseModel) {
			return entry.Models[i].IsCompat
		}
	}
	return false
}

func (m *Manager) attachResolvedAPIKeyModelInfo(req shinwayexecutor.Request, auth *Auth, routeModel, upstreamModel string) shinwayexecutor.Request {
	return attachResolvedAPIKeyModelInfo(m.loadAPIKeyModelRouting(), req, auth, routeModel, upstreamModel)
}

func attachResolvedAPIKeyModelInfo(routing *apiKeyModelRoutingSnapshot, req shinwayexecutor.Request, auth *Auth, routeModel, upstreamModel string) shinwayexecutor.Request {
	if auth == nil || auth.AuthKind() != AuthKindAPIKey {
		return req
	}
	if routing == nil {
		return req
	}
	table := routing.capabilities
	byRoute := table[strings.TrimSpace(auth.ID)]
	if len(byRoute) == 0 {
		return req
	}
	requestedModel := rewriteModelForAuth(strings.TrimSpace(routeModel), auth)
	selected := strings.TrimSpace(upstreamModel)
	_, candidates := modelAliasLookupCandidates(requestedModel)
	// An explicit configured suffix must win over the base alias candidate. This
	// keeps a request such as model(low) from inheriting model(high)'s settings.
	if requestSuffix := thinking.ParseSuffix(requestedModel); requestSuffix.HasSuffix {
		candidates = []string{requestedModel, requestSuffix.ModelName}
	}
	for _, candidate := range candidates {
		for _, route := range byRoute[strings.ToLower(strings.TrimSpace(candidate))] {
			if strings.EqualFold(strings.TrimSpace(route.upstreamModel), selected) || configuredUpstreamFallbackMatches(route.upstreamModel, selected) {
				metadata := make(map[string]any, len(req.Metadata)+1)
				maps.Copy(metadata, req.Metadata)
				metadata[resolvedAPIKeyModelInfoMetadataKey] = route.modelInfo
				req.Metadata = metadata
				return req
			}
		}
	}
	return req
}

func configuredUpstreamFallbackMatches(configured, selected string) bool {
	configuredResult := thinking.ParseSuffix(strings.TrimSpace(configured))
	if configuredResult.HasSuffix {
		return false
	}
	selectedResult := thinking.ParseSuffix(strings.TrimSpace(selected))
	return strings.EqualFold(strings.TrimSpace(configuredResult.ModelName), strings.TrimSpace(selectedResult.ModelName))
}

func (m *Manager) rebuildAPIKeyModelCapabilitiesLocked(cfg *internalconfig.Config) {
	if m == nil {
		return
	}
	if cfg == nil {
		cfg = &internalconfig.Config{}
	}
	out := make(apiKeyModelCapabilityTable)
	for _, auth := range m.auths {
		if auth == nil || strings.TrimSpace(auth.ID) == "" || auth.AuthKind() != AuthKindAPIKey {
			continue
		}
		byRoute := compileAPIKeyModelCapabilitiesForAuth(cfg, auth)
		if len(byRoute) > 0 {
			out[auth.ID] = byRoute
		}
	}
}

func compileAPIKeyModelCapabilitiesForAuth(cfg *internalconfig.Config, auth *Auth) map[string][]apiKeyModelCapabilityRoute {
	if cfg == nil || auth == nil || auth.AuthKind() != AuthKindAPIKey {
		return nil
	}
	out := make(map[string][]apiKeyModelCapabilityRoute)
	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "gemini":
		if entry := resolveGeminiAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "gemini")
		}
	case "gemini-interactions":
		if entry := resolveInteractionsAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "interactions")
		}
	case "claude":
		if entry := resolveClaudeAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "claude")
		}
	case "codex":
		if entry := resolveCodexAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "codex")
		}
	case "xai":
		if entry := resolveXAIAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "xai")
		}
	case "nvidia":
		if entry := resolveNVIDIAAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "nvidia")
		}
	case "zen":
		if entry := resolveZenAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "zen")
		}
	case "vertex":
		if entry := resolveVertexAPIKeyConfig(cfg, auth); entry != nil {
			compileConfiguredModelCapabilities(out, entry.Models, "gemini")
		}
	default:
		providerKey, compatName := "", ""
		if auth.Attributes != nil {
			providerKey, compatName = strings.TrimSpace(auth.Attributes["provider_key"]), strings.TrimSpace(auth.Attributes["compat_name"])
		}
		if entry := resolveOpenAICompatConfig(cfg, providerKey, compatName, auth.Provider); entry != nil {
			compileOpenAICompatibleModelCapabilities(out, entry.Models)
		}
	}
	return out
}

func compileConfiguredModelCapabilities[T interface {
	GetName() string
	GetAlias() string
	GetThinking() *registry.ThinkingSupport
}](out map[string][]apiKeyModelCapabilityRoute, models []T, modelType string) {
	for i := range models {
		isCompat := false
		if compatible, ok := any(models[i]).(interface{ GetIsCompat() bool }); ok {
			isCompat = compatible.GetIsCompat()
		}
		maxContextLength := 0
		if contextual, ok := any(models[i]).(interface{ GetMaxContextLength() int }); ok {
			maxContextLength = contextual.GetMaxContextLength()
		}
		addConfiguredModelCapability(out, models[i].GetName(), models[i].GetAlias(), modelType, models[i].GetThinking(), isCompat, maxContextLength)
	}
}

func compileOpenAICompatibleModelCapabilities(out map[string][]apiKeyModelCapabilityRoute, models []internalconfig.OpenAICompatibilityModel) {
	for i := range models {
		support := models[i].Thinking
		if support == nil && !models[i].Image {
			support = &registry.ThinkingSupport{Levels: []string{"low", "medium", "high"}}
		}
		addConfiguredModelCapability(out, models[i].Name, models[i].Alias, "openai-compatibility", support, models[i].IsCompat, models[i].MaxContextLength)
	}
}

func addConfiguredModelCapability(out map[string][]apiKeyModelCapabilityRoute, name, alias, modelType string, support *registry.ThinkingSupport, isCompat bool, maxContextLength int) {
	name, alias = strings.TrimSpace(name), strings.TrimSpace(alias)
	if name == "" {
		name = alias
	}
	if alias == "" {
		alias = name
	}
	if name == "" {
		return
	}
	baseName := strings.TrimSpace(thinking.ParseSuffix(name).ModelName)
	modelInfo := registry.LookupStaticModelInfo(baseName)
	if modelInfo == nil {
		modelInfo = &registry.ModelInfo{}
	}
	modelInfo.ID, modelInfo.Type, modelInfo.UserDefined = name, strings.TrimSpace(modelType), false
	if support != nil {
		cloned := *support
		cloned.Levels = append([]string(nil), support.Levels...)
		modelInfo.Thinking = &cloned
	}
	modelInfo.IsCompat, modelInfo.MaxContextLength = isCompat, maxContextLength
	route := apiKeyModelCapabilityRoute{upstreamModel: name, modelInfo: modelInfo}
	seen := make(map[string]struct{})
	for _, routeModel := range []string{alias, name} {
		_, candidates := modelAliasLookupCandidates(routeModel)
		for _, candidate := range candidates {
			key := strings.ToLower(strings.TrimSpace(candidate))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			duplicate := false
			for _, existing := range out[key] {
				if strings.EqualFold(existing.upstreamModel, route.upstreamModel) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				out[key] = append(out[key], route)
			}
		}
	}
}
