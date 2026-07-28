package gemini

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Gemini,
		Claude,
		ConvertGeminiRequestToClaude,
		interfaces.TranslateResponse{
			Stream:     ConvertClaudeResponseToGemini,
			NonStream:  ConvertClaudeResponseToGeminiNonStream,
			TokenCount: GeminiTokenCount,
		},
	)
}
