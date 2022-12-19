package admin

import log "github.com/sirupsen/logrus"

func (in *internal) on_slot(slot uint64) {
	in.slot = slot
	in.attempt_add_period()
}

type PayoutEventType = int

const (
	EVENT_START                    PayoutEventType = 0
	EVENT_FINISH                   PayoutEventType = 1
	EVENT_CLOSE_OUT                PayoutEventType = 2
	EVENT_BID_CLOSED               PayoutEventType = 3
	EVENT_BID_FINAL                PayoutEventType = 4
	EVENT_VALIDATOR_IS_ADDING      PayoutEventType = 5
	EVENT_VALIDATOR_HAVE_WITHDRAWN PayoutEventType = 6
)

const PAYOUT_POST_FINISH_DELAY uint64 = 100

type PayoutEvent struct {
	Slot          uint64
	IsStateChange bool // true means the state changed; false means this is the existing state
	Type          PayoutEventType
}

func pevent(eventType PayoutEventType, isStateChange bool, slot uint64) PayoutEvent {
	return PayoutEvent{
		Slot:          0,
		IsStateChange: isStateChange,
		Type:          eventType,
	}
}

func (pi *payoutInternal) on_start(isTransition bool) error {
	pi.hasStarted = true
	if isTransition {
		log.Debugf("payout=%s period has started (slot=%d)", pi.payout.Id.String(), pi.slot)
		return pi.run_crank()
	} else {
		log.Debugf("payout=%s period has already started", pi.payout.Id.String())
	}
	return nil
}

func (pi *payoutInternal) on_finish(isTransition bool) error {
	pi.hasStarted = true
	pi.hasFinished = true
	if isTransition {
		log.Debugf("payout=%s period has finished (slot=%d)", pi.payout.Id.String(), pi.slot)
		return pi.run_close_bids()
	} else {
		log.Debugf("payout=%s period has already finished", pi.payout.Id.String())
	}
	return nil
}

// it is now possible to send a ClosePayout instruction
func (pi *payoutInternal) on_close_out(isTransition bool) error {
	pi.isReadyToClose = true
	if isTransition {
		return pi.run_close_payout()
	}
	return nil
}

func (pi *payoutInternal) on_bid_final(isTransition bool) error {
	pi.bidIsFinal = true
	if isTransition && pi.cancelCrank == nil {
		pi.cancelCrank()
		pi.cancelCrank = nil
	}

	return nil
}

func (pi *payoutInternal) on_bid_closed(isTransition bool) error {
	pi.bidHasClosed = true

	if isTransition {
		return pi.run_close_payout()
	}

	return nil
}

func (pi *payoutInternal) on_validator_is_adding(isTransition bool) error {
	pi.validatorAddingHasStarted = true
	if isTransition {
		log.Debugf("payout=%s is adding validators", pi.payout.Id.String())
	}
	return nil
}

func (pi *payoutInternal) on_validator_have_withdrawn(isTransition bool) error {
	pi.validatorAddingHasStarted = true
	pi.validatorAddingIsDone = true
	if isTransition {
		return pi.run_close_payout()
	}
	return nil
}
