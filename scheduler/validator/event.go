package validator

import (
	"context"

	pyt "github.com/solpipe/solpipe-tool/state/payout"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type TriggerValidator struct {
	Context   context.Context
	Validator val.Validator
	Payout    pyt.Payout
	Receipt   rpt.Receipt
}
