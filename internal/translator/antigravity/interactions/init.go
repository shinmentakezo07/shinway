package interactions

import (
	. "github.com/shinmentakezo07/shinway/v7/internal/constant"
	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
	"github.com/shinmentakezo07/shinway/v7/internal/translator/translator"
)

func init() {
	translator.Register(
		Interactions,
		Antigravity,
		ConvertInteractionsRequestToAntigravity,
		interfaces.TranslateResponse{
			Stream:    ConvertAntigravityResponseToInteractions,
			NonStream: ConvertAntigravityResponseToInteractionsNonStream,
		},
	)
}
