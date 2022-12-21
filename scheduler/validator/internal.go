package validator

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type internal struct {
	ctx                      context.Context
	errorC                   chan<- error
	eventC                   chan<- sch.Event
	closeSignalCList         []chan<- error
	clockPeriodStartC        chan bool
	clockPeriodPostCloseC    chan bool
	trackHome                *dssub.SubHome[sch.Event]
	pwd                      pipe.PayoutWithData
	v                        val.Validator
	data                     cba.ValidatorManager
	receiptData              *cba.Receipt
	receipt                  rpt.Receipt
	cancelValidatorSetPayout context.CancelFunc
	cancelStake              context.CancelFunc
	cancelValidatorWithraw   context.CancelFunc
	validatorHasAdded        bool
	validatorHasWithdrawn    bool
}

func (in *internal) Payout() pyt.Payout {
	return in.pwd.Payout
}

func (in *internal) Validator() val.Validator {
	return in.v
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	v val.Validator,
	trackHome *dssub.SubHome[sch.Event],
) {
	defer cancel()

	errorC := make(chan error, 1)
	rC := make(chan receiptWithTransition)
	doneC := ctx.Done()
	receiptEventC := make(chan sch.Event)
	clockPeriodStartC := make(chan bool, 1)
	clockPeriodPostCloseC := make(chan bool, 1)
	paymentEventSub := ps.OnEvent()
	defer paymentEventSub.Unsubscribe()
	receiptSub := v.OnReceipt()
	defer receiptSub.Unsubscribe()

	var err error
	var event sch.Event

	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.trackHome = trackHome
	defer trackHome.Close()
	in.errorC = errorC
	in.eventC = receiptEventC
	in.clockPeriodStartC = clockPeriodStartC
	in.clockPeriodPostCloseC = clockPeriodPostCloseC
	in.pwd = pwd
	in.v = v
	in.validatorHasAdded = false
	in.validatorHasWithdrawn = false

	in.data, err = v.Data()
	if err != nil {
		in.errorC <- err
	}
	var ctxValidatorSetPayout context.Context
	ctxValidatorSetPayout, in.cancelValidatorSetPayout = context.WithCancel(ctx)
	go loopOpenReceipt(
		ctxValidatorSetPayout,
		in.cancelValidatorSetPayout,
		in.errorC,
		rC,
		in.eventC,
		in.pwd.Payout,
		v,
		in.clockPeriodStartC,
	)

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case id := <-trackHome.DeleteC:
			trackHome.Delete(id)
		case r := <-trackHome.ReqC:
			trackHome.Receive(r)
		case err = <-paymentEventSub.ErrorC:
			break out
		case req := <-internalC:
			req(in)
		case event = <-paymentEventSub.StreamC:
			in.on_payout_event(event)
		case rwd := <-rC:
			in.on_receipt(rwd)
		case event = <-receiptEventC:
			in.on_receipt_event(event)
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *internal) on_payout_event(event sch.Event) {
	switch event.Type {
	case sch.EVENT_PERIOD_START:
		if in.cancelValidatorSetPayout != nil {
			in.cancelValidatorSetPayout()
			in.cancelValidatorSetPayout = nil
		}
		if in.receiptData == nil {
			// we have not received receipt data
			in.clockPeriodStartC <- event.IsStateChange
		}
	case sch.EVENT_DELAY_CLOSE_PAYOUT:
		in.clockPeriodPostCloseC <- event.IsStateChange
	case sch.EVENT_VALIDATOR_HAVE_WITHDRAWN:
		// exit the loop
		if in.cancelValidatorWithraw != nil {
			in.cancelValidatorWithraw()
			in.cancelValidatorWithraw = nil
		}
		in.errorC <- nil
	default:
	}
}

func (in *internal) on_receipt_event(event sch.Event) {
	if in.cancelValidatorSetPayout != nil {
		in.cancelValidatorSetPayout()
	}
	switch event.Type {
	case sch.EVENT_STAKER_IS_ADDING:
		in.trackHome.Broadcast(event)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN_EMPTY:
		in.run_validator_withdraw(event.IsStateChange)
	case sch.EVENT_STAKER_HAVE_WITHDRAWN:
		in.run_validator_withdraw(event.IsStateChange)
	default:
		in.errorC <- errors.New("unknown event")
	}
}

func (in *internal) run_validator_withdraw(isStateChange bool) {
	var ctxC context.Context
	ctxC, in.cancelValidatorWithraw = context.WithCancel(in.ctx)
	in.trackHome.Broadcast(sch.CreateWithPayload(
		sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT,
		isStateChange,
		0,
		&TriggerValidator{
			Context:   ctxC,
			Payout:    in.pwd.Payout,
			Validator: in.v,
			Receipt:   in.receipt,
		},
	))
}
