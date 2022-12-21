package payout

import (
	log "github.com/sirupsen/logrus"
)

func (in *internal) on_pre_start(isTransition bool) {
	if isTransition {
		in.run_validator_set_payout()
	}
}

func (in *internal) on_start(isTransition bool) {
	in.hasStarted = true
	if isTransition {
		log.Debugf("payout=%s period has started", in.payout.Id.String())
		in.run_crank()
		if in.cancelValidatorSetPayout != nil {
			in.cancelValidatorSetPayout()
		}
	} else {
		log.Debugf("payout=%s period has already started", in.payout.Id.String())
	}

}

func (in *internal) on_finish(isTransition bool) {
	in.hasStarted = true
	in.hasFinished = true
	if isTransition {
		log.Debugf("payout=%s period has finished", in.payout.Id.String())
		// this can be run as soon as on_bid_final is run; but for simplicity, we wait until the period is complete
		in.run_close_bids()
	} else {
		log.Debugf("payout=%s period has already finished", in.payout.Id.String())
	}
}

// it is now possible to send a ClosePayout instruction
func (in *internal) on_clock_close_payout(isTransition bool) {
	in.isClockReadyToClose = true
	if isTransition {
		in.run_close_payout()
	}
}

func (in *internal) on_bid_final(isTransition bool) {
	in.bidIsFinal = true
	if isTransition && in.cancelCrank == nil {
		in.cancelCrank()
		in.cancelCrank = nil
	}

}

func (in *internal) on_bid_closed(isTransition bool) {
	in.bidIsFinal = true
	in.bidHasClosed = true

	if isTransition {
		if in.cancelCloseBid == nil {
			in.cancelCloseBid()
			in.cancelCloseBid = nil
		}
		in.run_close_payout()
	}
}

func (in *internal) on_validator_is_adding(isTransition bool) {
	in.validatorAddingHasStarted = true
	if isTransition {
		log.Debugf("payout=%s is adding validators", in.payout.Id.String())
	}

}

func (in *internal) on_validator_have_withdrawn(isTransition bool) {
	in.validatorAddingHasStarted = true
	in.validatorAddingIsDone = true
	if isTransition {
		in.run_close_payout()
	}
}

func (in *internal) on_staker_is_adding(isTransition bool) {
	in.stakerAddingHasStarted = true

}

func (in *internal) on_staker_have_withdrawn(isTransition bool) {
	in.stakerAddingHasStarted = true
	in.stakerAddingIsDone = true
	if isTransition {
		in.run_validator_withdraw()
	}
}
