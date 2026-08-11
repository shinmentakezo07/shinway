package helps

import (
	"github.com/shinmentakezo07/shinway/v7/internal/thinking"
	shinwayauth "github.com/shinmentakezo07/shinway/v7/sdk/shinway/auth"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

// APIKeyModelIsCompat reports whether the selected configured API-key model opts
// into compatibility handling for signed thinking blocks.
func APIKeyModelIsCompat(req shinwayexecutor.Request) bool {
	modelInfo, ok := shinwayauth.ResolvedAPIKeyModelInfo(req)
	return ok && modelInfo.IsCompat
}

// ApplyRequestThinking applies the precise selected API-key model capability
// snapshot when available, otherwise retains normal registry-based behavior.
func ApplyRequestThinking(body []byte, req shinwayexecutor.Request, opts shinwayexecutor.Options, fromFormat, toFormat, provider string) ([]byte, error) {
	if modelInfo, ok := shinwayauth.ResolvedAPIKeyModelInfo(req); ok {
		return thinking.ApplyThinkingWithModelInfo(body, req.Model, fromFormat, toFormat, provider, modelInfo)
	}
	return thinking.ApplyThinking(body, req.Model, fromFormat, toFormat, provider)
}
