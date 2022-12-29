package validator

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
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

func (in *internal) on_payout_event(event sch.Event) {
	log.Debugf("on_payout_event: %s", event.String())
	switch event.Type {
	case sch.TRIGGER_VALIDATOR_SET_PAYOUT:
		log.Debugf("setting validator")
		in.trackHome.Broadcast(event)
	case sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT:
		log.Debugf("withdrawing validator")
		if in.cancelValidatorWithraw == nil {
			in.ctxValidatorWithraw, in.cancelValidatorWithraw = context.WithCancel(in.ctx)
			in.isValidatorWithdrawTransition = event.IsStateChange
			in.run_validator_withdraw()
		} else {
			in.errorC <- errors.New("duplicate withdraw trigger")
		}
	case sch.EVENT_PERIOD_START:
		log.Debug("event_period_start")
		// do not cancel context here or else we will never get a receipt
		//if in.cancelValidatorSetPayout != nil {
		//	in.cancelValidatorSetPayout()
		//	in.cancelValidatorSetPayout = nil
		//}
		if in.receiptData == nil {
			// we have not received receipt data
			in.clockPeriodStartC <- event.IsStateChange
		}
	case sch.EVENT_DELAY_CLOSE_PAYOUT:
		in.clockPeriodPostCloseC <- event.IsStateChange
	default:
		log.Debugf("on_payout_event unknown event: %s", event.String())
	}
}

func (in *internal) run_validator_withdraw() {
	if in.receiptData == nil {
		return
	}
	if in.ctxValidatorWithraw == nil {
		return
	}
	in.trackHome.Broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		in.isValidatorWithdrawTransition,
		0,
		in.trigger(in.ctxValidatorWithraw),
	))
}

// track stakers and validator receipt withdrawal
func (in *internal) on_receipt_event(event sch.Event) {
	log.Debugf("on_receipt_event: %s", event.String())
	switch event.Type {
	case sch.EVENT_STAKER_IS_ADDING:
		in.trackHome.Broadcast(event)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN:
		in.trackHome.Broadcast(event)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN_EMPTY:
		in.trackHome.Broadcast(event)
	case sch.EVENT_VALIDATOR_HAVE_WITHDRAWN:
		// the validator's relationship with payout is over
		// the receipt account has closed
		if in.cancelValidatorWithraw != nil {
			in.cancelValidatorWithraw()
			in.cancelValidatorWithraw = nil
		}
		in.trackHome.Broadcast(event)
		in.errorC <- nil
	default:
		log.Debugf("on_receipt_event unknown event: %s", event.String())
	}
}
