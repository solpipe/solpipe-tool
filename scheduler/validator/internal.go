package validator

import (
	"context"

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
	ctx                           context.Context
	pipeline                      pipe.Pipeline
	errorC                        chan<- error
	eventC                        chan<- sch.Event
	closeSignalCList              []chan<- error
	clockPeriodStartC             chan<- bool
	clockPeriodPostCloseC         chan bool
	trackHome                     *dssub.SubHome[sch.Event]
	pwd                           pipe.PayoutWithData
	validator                     val.Validator
	data                          cba.ValidatorManager
	receiptData                   *cba.Receipt
	receipt                       rpt.Receipt
	cancelValidatorSetPayout      context.CancelFunc
	cancelStake                   context.CancelFunc
	ctxValidatorWithraw           context.Context
	cancelValidatorWithraw        context.CancelFunc
	isValidatorWithdrawTransition bool
	validatorHasAdded             bool
	validatorHasWithdrawn         bool
}

func (in *internal) Payout() pyt.Payout {
	return in.pwd.Payout
}

func (in *internal) Validator() val.Validator {
	return in.validator
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pipeline pipe.Pipeline,
	pwd pipe.PayoutWithData,
	pipelineSchedule sch.Schedule,
	payoutSchedule sch.Schedule,
	v val.Validator,
	trackHome *dssub.SubHome[sch.Event],
) {
	defer cancel()

	errorC := make(chan error, 1)
	receiptC := make(chan receiptWithTransition)
	doneC := ctx.Done()
	receiptEventC := make(chan sch.Event)
	clockPeriodStartC := make(chan bool, 1)
	clockPeriodPostCloseC := make(chan bool, 1)
	payoutEventSub := payoutSchedule.OnEvent()
	defer payoutEventSub.Unsubscribe()
	receiptSub := v.OnReceipt()
	defer receiptSub.Unsubscribe()

	var err error
	var event sch.Event

	in := new(internal)
	in.ctx = ctx
	in.pipeline = pipeline
	in.closeSignalCList = make([]chan<- error, 0)
	in.trackHome = trackHome
	defer trackHome.Close()
	in.errorC = errorC
	in.eventC = receiptEventC
	in.isValidatorWithdrawTransition = false
	in.clockPeriodStartC = clockPeriodStartC
	in.clockPeriodPostCloseC = clockPeriodPostCloseC
	in.pwd = pwd
	in.validator = v
	in.validatorHasAdded = false
	in.validatorHasWithdrawn = false

	in.data, err = v.Data()
	if err != nil {
		in.errorC <- err
	}
	if in.pwd.Id.String() == "B2weAoPHiLYtANsEPXzsmDrKjoNB1bRs2QZNXat64LiF" {
		log.Debugf("got target")
	}

	var ctxValidatorSetPayout context.Context
	ctxValidatorSetPayout, in.cancelValidatorSetPayout = context.WithCancel(ctx)
	log.Debugf("open receipt payout=%s", in.pwd.Id.String())
	go loopOpenReceipt(
		ctxValidatorSetPayout,
		in.cancelValidatorSetPayout,
		in.errorC,
		in.pipeline,
		receiptC,
		in.pwd.Payout,
		v,
		clockPeriodStartC,
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
		case err = <-payoutEventSub.ErrorC:
			break out
		case event = <-payoutEventSub.StreamC:
			in.on_payout_event(event)
		case req := <-internalC:
			req(in)
		case rwd := <-receiptC:
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

/*
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
			Pipeline:  in.pipeline,
		},
	))
}*/

func (in *internal) trigger(ctx context.Context) *TriggerValidator {
	log.Debugf("receipt=%+v", in.receipt)
	log.Debugf("receipt id=%s", in.receipt.Id.String())
	return &TriggerValidator{
		Context:   ctx,
		Validator: in.validator,
		Pipeline:  in.pipeline,
		Payout:    in.pwd.Payout,
		Receipt:   in.receipt,
	}
}
