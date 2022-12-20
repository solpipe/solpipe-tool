package payout

import (
	"context"
	"errors"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
)

type Trigger struct {
	Context context.Context // context Done() triggered when window to execute disappears
	Payout  pyt.Payout
}

func ReadTrigger(event sch.Event) (*Trigger, error) {
	trigger, ok := event.Payload.(*Trigger)
	if !ok {
		return nil, errors.New("no trigger in payload")
	}
	return trigger, nil
}

func (in *internal) generic_trigger() (*Trigger, context.CancelFunc) {
	ctxC, cancel := context.WithCancel(in.ctx)
	return &Trigger{
		Context: ctxC,
		Payout:  in.payout,
	}, cancel
}

// this function must only be called once!
func (in *internal) run_crank() {

	if !in.hasStarted {
		return
	}
	if in.bidIsFinal || in.bidHasClosed {
		return
	}
	var trigger *Trigger
	trigger, in.cancelCrank = in.generic_trigger()
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_CRANK,
		true,
		0,
		trigger,
	))
}

func (in *internal) run_close_bids() {

	var trigger *Trigger
	trigger, in.cancelCloseBid = in.generic_trigger()
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_CLOSE_BIDS,
		true,
		0,
		trigger,
	))

}

func (in *internal) run_close_payout() {
	if !in.bidHasClosed {
		return
	}
	if !in.isClockReadyToClose {
		return
	}
	if !in.validatorAddingIsDone {
		return
	}
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_CLOSE_PAYOUT,
		true,
		0,
		&Trigger{
			Context: nil, // nil here because the scheduler will close after broadcasting this event
			Payout:  in.payout,
		},
	))
	in.errorC <- nil

}

// stakers can add once the context in the trigger has Done() fired
func (in *internal) run_validator_set_payout() {

	var trigger *Trigger
	trigger, in.cancelValidatorSetPayout = in.generic_trigger()
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_VALIDATOR_SET_PAYOUT,
		true,
		0,
		trigger,
	))
}

func (in *internal) run_validator_withdraw() {
	if in.validatorAddingIsDone {
		return
	}

	var trigger *Trigger
	trigger, in.cancelValidatorWithdraw = in.generic_trigger()
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		true,
		0,
		trigger,
	))
}

func (in *internal) run_staker_withdraw() {
	if in.stakerAddingIsDone {
		return
	}
	var trigger *Trigger
	trigger, in.cancelStakerWithdraw = in.generic_trigger()
	in.eventHome.Broadcast(sch.CreateWithPayload(
		TRIGGER_STAKER_WITHDRAW,
		true,
		0,
		trigger,
	))
}
