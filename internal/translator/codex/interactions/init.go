package interactions

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Interactions,
		Codex,
		ConvertInteractionsRequestToCodex,
		interfaces.TranslateResponse{
			Stream:    ConvertCodexResponseToInteractions,
			NonStream: ConvertCodexResponseToInteractionsNonStream,
		},
	)
}
