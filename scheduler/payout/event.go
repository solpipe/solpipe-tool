package payout

import (
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
)

const (
	EVENT_PRE_START                    sch.EventType = -1
	EVENT_START                        sch.EventType = 0
	EVENT_FINISH                       sch.EventType = 1
	EVENT_CLOSE_OUT                    sch.EventType = 2
	EVENT_BID_CLOSED                   sch.EventType = 3
	EVENT_BID_FINAL                    sch.EventType = 4
	EVENT_VALIDATOR_IS_ADDING          sch.EventType = 5
	EVENT_VALIDATOR_HAVE_WITHDRAWN     sch.EventType = 6
	EVENT_STAKER_IS_ADDING             sch.EventType = 7
	EVENT_STAKER_HAVE_WITHDRAWN        sch.EventType = 8
	TRIGGER_CRANK                      sch.EventType = 100
	TRIGGER_CLOSE_BIDS                 sch.EventType = 101
	TRIGGER_CLOSE_PAYOUT               sch.EventType = 102
	TRIGGER_VALIDATOR_SET_PAYOUT       sch.EventType = 103
	TRIGGER_VALIDATOR_WITHDRAW_RECEIPT sch.EventType = 104
	TRIGGER_STAKER_ADD                 sch.EventType = 105
	TRIGGER_STAKER_WITHDRAW            sch.EventType = 106
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
		in.run_close_bids()
	} else {
		log.Debugf("payout=%s period has already finished", in.payout.Id.String())
	}

}

// it is now possible to send a ClosePayout instruction
func (in *internal) on_close_out(isTransition bool) {
	in.isReadyToClose = true
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
	in.bidHasClosed = true

	if isTransition {
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
