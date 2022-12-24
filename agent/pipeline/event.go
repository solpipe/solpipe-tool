package pipeline

import (
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
)

func (in *internal) on_pipeline_event(event sch.Event) {
	var err error
	switch event.Type {
	case sch.TRIGGER_PERIOD_APPEND:
		log.Debugf("ignoring period append event=%s", event.String())
		err = in.run_period_append(event)
	default:
		log.Debugf("no match for event=%s", event.String())
	}
	if err != nil {
		in.errorC <- err
	}
}

func (in *internal) on_payout_event(event sch.Event) {
	var err error
	payload, err := schpyt.ReadTrigger(event)
	if err != nil {
		in.errorC <- err
		return
	}

	switch event.Type {
	case sch.TRIGGER_CRANK:
		log.Debugf("payout=%s crank event=%s", payload.Payout.Id.String(), event.String())
		err = in.run_payout_crank(event)
	case sch.TRIGGER_CLOSE_BIDS:
		log.Debugf("payout=%s close bids event=%s", payload.Payout.Id.String(), event.String())
		err = in.run_payout_close_bids(event)
	case sch.TRIGGER_CLOSE_PAYOUT:
		log.Debugf("payout=%s close payout event=%s", payload.Payout.Id.String(), event.String())
		err = in.run_payout_close_payout(event)
	default:
		log.Debugf("payout=%s unfiltered event=%s", payload.Payout.Id.String(), event.String())
	}
	if err != nil {
		in.errorC <- err
	}
}
