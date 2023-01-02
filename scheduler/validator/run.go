package validator

import (
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
)

// protection from running this function multiple times comes from schpyt
func (in *internal) run_validator_set_payout(event sch.Event) {
	oldTrigger, err := schpyt.ReadTrigger(event)
	if err != nil {
		in.errorC <- err
		return
	}

	log.Debugf("setting validator")
	newEvent := sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_SET_PAYOUT,
		false,
		0,
		in.trigger(oldTrigger.Context),
	)
	in.broadcast(newEvent)
}

func (in *internal) run_validator_withdraw() {
	if in.receiptData == nil {
		return
	}
	if in.ctxValidatorWithraw == nil {
		return
	}
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		in.isValidatorWithdrawTransition,
		0,
		in.trigger(in.ctxValidatorWithraw),
	))
}
