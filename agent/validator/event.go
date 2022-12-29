package validator

import (
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
)

func (in *internal) on_event(event sch.Event) error {
	var err error
	switch event.Type {
	case sch.TRIGGER_VALIDATOR_SET_PAYOUT:
		err = in.run_validator_set_payout(event)
	case sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT:
		err = in.run_validator_withdraw_receipt(event)
	default:
		log.Debugf("received unmatched event: %s", event.String())
	}
	return err
}
