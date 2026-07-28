package chat_completions

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		OpenAI,
		Antigravity,
		ConvertOpenAIRequestToAntigravity,
		interfaces.TranslateResponse{
			Stream:    ConvertAntigravityResponseToOpenAI,
			NonStream: ConvertAntigravityResponseToOpenAINonStream,
		},
	)
}
