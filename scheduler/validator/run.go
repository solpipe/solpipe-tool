package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
)

// protection from running this function multiple times comes from schpyt
func (in *internal) run_validator_set_payout(ctxC context.Context) {

	log.Debugf("setting validator")
	newEvent := sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_SET_PAYOUT,
		false,
		0,
		in.trigger(ctxC),
	)
	if 0 < in.eventHome.SubscriberCount() {
		in.eventHome.Broadcast(newEvent)
	} else {
		in.preStartEvent = &newEvent
	}

}

func (in *internal) run_validator_withdraw() {
	if in.receiptData == nil {
		return
	}
	if in.ctxValidatorWithraw == nil {
		return
	}
	in.eventHome.Broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		in.isValidatorWithdrawTransition,
		0,
		in.trigger(in.ctxValidatorWithraw),
	))
}
