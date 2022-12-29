package payout

import (
	"errors"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
)

func (in *internal) on_event(event sch.Event) error {

	log.Debugf("event payout=%s  %s", in.payout.Id.String(), event.String())

	switch event.Type {
	case sch.EVENT_PERIOD_PRE_START:
		in.on_pre_start(event.IsStateChange)
	case sch.EVENT_PERIOD_START:
		in.on_start(event.IsStateChange)
	case sch.EVENT_PERIOD_FINISH:
		in.on_finish(event.IsStateChange)
	case sch.EVENT_DELAY_CLOSE_PAYOUT:
		in.on_clock_close_payout(event.IsStateChange)
	case sch.EVENT_BID_CLOSED:
		in.on_bid_closed(event.IsStateChange)
	case sch.EVENT_BID_FINAL:
		in.on_bid_final(event.IsStateChange)
	case sch.EVENT_VALIDATOR_IS_ADDING:
		in.on_validator_is_adding(event.IsStateChange)
	case sch.EVENT_VALIDATOR_HAVE_WITHDRAWN:
		in.on_validator_have_withdrawn(event.IsStateChange)
	case sch.EVENT_STAKER_IS_ADDING:
		in.on_staker_is_adding(event.IsStateChange)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN:
		in.on_staker_have_withdrawn(event.IsStateChange)
	default:
		return errors.New("unknown event")
	}
	if !event.IsTrigger() {
		in.eventHome.Broadcast(event)
	}

	return nil
}

func (in *internal) on_pre_start(isTransition bool) {
	in.run_validator_set_payout()
}

// clock: start
func (in *internal) on_start(isTransition bool) {
	in.hasStarted = true
	log.Debugf("payout=%s period has started", in.payout.Id.String())
	if in.cancelValidatorSetPayout != nil {
		in.cancelValidatorSetPayout()
	}
	if !in.bidIsFinal {
		in.run_crank()
	}
}

// clock: finish
func (in *internal) on_finish(isTransition bool) {
	if !in.hasStarted {
		in.on_start(false)
	}

	in.hasFinished = true

	if !in.bidHasClosed {
		log.Debugf("payout=%s period has finished", in.payout.Id.String())
		// this can be run as soon as on_bid_final is run; but for simplicity, we wait until the period is complete
		in.run_close_bids()
	}
}

// clock: 100 slots after finish
// it is now possible to send a ClosePayout instruction
func (in *internal) on_clock_close_payout(isTransition bool) {

	in.isClockReadyToClose = true
	log.Debugf("payout=%s time to close payout", in.payout.Id.String())
	in.run_close_payout()
}

func (in *internal) on_bid_final(isTransition bool) {
	in.bidIsFinal = true
	log.Debugf("payout=%s bid has been finalized", in.payout.Id.String())
	if in.cancelCrank != nil {
		in.cancelCrank()
		in.cancelCrank = nil
	}
}

func (in *internal) on_bid_closed(isTransition bool) {
	in.bidIsFinal = true
	in.bidHasClosed = true
	log.Debugf("payout=%s bid has been closed", in.payout.Id.String())

	if in.cancelCloseBid != nil {
		in.cancelCloseBid()
		in.cancelCloseBid = nil
	}
	in.run_close_payout()
}

func (in *internal) on_validator_is_adding(isTransition bool) {
	in.validatorAddingHasStarted = true
	log.Debugf("payout=%s is adding validators", in.payout.Id.String())

}

func (in *internal) on_validator_have_withdrawn(isTransition bool) {
	in.validatorAddingHasStarted = true
	in.validatorHasWithdrawn = true
	log.Debugf("payout=%s validator has withdrawn", in.payout.Id.String())

	in.run_close_payout()
}

func (in *internal) on_staker_is_adding(isTransition bool) {
	in.stakerAddingHasStarted = true

}

func (in *internal) on_staker_have_withdrawn(isTransition bool) {
	in.stakerAddingHasStarted = true
	in.stakerAddingIsDone = true
	in.run_validator_withdraw()
}
