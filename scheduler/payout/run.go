package payout

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
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
	if in.cancelCrank != nil {
		return
	}
	log.Debugf("payout=%s run crank", in.payout.Id.String())
	var trigger *Trigger
	trigger, in.cancelCrank = in.generic_trigger()
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_CRANK,
		true,
		0,
		trigger,
	))
}

func (in *internal) run_close_bids() {
	if in.bidHasClosed {
		return
	}
	if in.cancelCloseBid != nil {
		return
	}
	log.Debugf("payout=%s run close bids", in.payout.Id.String())
	var trigger *Trigger
	trigger, in.cancelCloseBid = in.generic_trigger()
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_CLOSE_BIDS,
		true,
		0,
		trigger,
	))

}

// run this function only once and close the scheduler
func (in *internal) run_close_payout() {
	if !in.bidHasClosed {
		log.Debugf("payout=%s bid has not closed yet", in.payout.Id.String())
		return
	}
	if !in.isClockReadyToClose {
		log.Debugf("payout=%s clock not ready to close", in.payout.Id.String())
		return
	}
	if !in.validatorHasWithdrawn {
		log.Debugf("payout=%s validator has not withdrawn", in.payout.Id.String())
		return
	}
	log.Debugf("payout=%s run close payout", in.payout.Id.String())
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_CLOSE_PAYOUT,
		true,
		0,
		&Trigger{
			Context: nil, // nil here because the scheduler will close after broadcasting this event
			Payout:  in.payout,
		},
	))

	in.keepPayoutOpen = false

}

// stakers can add once the context in the trigger has Done() fired
func (in *internal) run_validator_set_payout() {
	if in.hasStarted {
		return
	}
	if in.cancelValidatorSetPayout != nil {
		return
	}
	log.Debugf("trigger set payout=%s", in.payout.Id.String())
	var trigger *Trigger
	trigger, in.cancelValidatorWithdraw = in.generic_trigger()
	go loopDebugReadTrigger(trigger.Context, in.payout.Id)
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_SET_PAYOUT,
		true,
		0,
		trigger,
	))

}

func loopDebugReadTrigger(
	ctx context.Context,
	payoutId sgo.PublicKey,
) {
	<-ctx.Done()
	log.Debugf("canceling validator set payout=%s", payoutId.String())
}

func (in *internal) run_validator_withdraw() {
	if in.validatorHasWithdrawn {
		return
	}

	var trigger *Trigger
	trigger, in.cancelValidatorWithdraw = in.generic_trigger()
	in.broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		true,
		0,
		trigger,
	))
}
