package validator

import sch "github.com/solpipe/solpipe-tool/scheduler"

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
