package validator

import (
	"context"
	"errors"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type TriggerValidator struct {
	Context   context.Context
	Validator val.Validator
	Payout    pyt.Payout
	Receipt   rpt.Receipt
	Pipeline  pipe.Pipeline
}

func ReadTrigger(
	event sch.Event,
) (*TriggerValidator, error) {
	if event.Payload == nil {
		return nil, errors.New("no payload")
	}
	d, ok := event.Payload.(*TriggerValidator)
	if !ok {
		return nil, errors.New("bad format for payload")
	}
	return d, nil

}
