package claude

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Claude,
		Interactions,
		ConvertClaudeRequestToInteractions,
		interfaces.TranslateResponse{
			Stream:    ConvertInteractionsResponseToClaude,
			NonStream: ConvertInteractionsResponseToClaudeNonStream,
		},
	)
}
