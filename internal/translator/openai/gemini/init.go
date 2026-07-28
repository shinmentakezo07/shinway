package gemini

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Gemini,
		OpenAI,
		ConvertGeminiRequestToOpenAI,
		interfaces.TranslateResponse{
			Stream:     ConvertOpenAIResponseToGemini,
			NonStream:  ConvertOpenAIResponseToGeminiNonStream,
			TokenCount: GeminiTokenCount,
		},
	)
}
