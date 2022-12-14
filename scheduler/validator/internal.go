package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
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
	clockPeriodStartC             chan bool
	sentPeriodStart               bool
	clockPeriodPostCloseC         chan bool
	sentPeriodPostClose           bool
	eventHome                     *dssub.SubHome[sch.Event]
	pwd                           pipe.PayoutWithData
	validator                     val.Validator
	data                          cba.ValidatorManager
	receiptData                   *cba.Receipt
	receipt                       rpt.Receipt
	receiptC                      chan<- receiptWithTransition
	cancelStake                   context.CancelFunc
	ctxValidatorWithraw           context.Context
	cancelValidatorWithraw        context.CancelFunc
	isValidatorWithdrawTransition bool
	validatorHasAdded             bool
	validatorHasWithdrawn         bool
	history                       *ll.Generic[sch.Event]
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
	eventHome *dssub.SubHome[sch.Event],
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
	in.eventHome = eventHome
	defer eventHome.Close()
	in.errorC = errorC
	in.eventC = receiptEventC
	in.receiptC = receiptC
	in.isValidatorWithdrawTransition = false
	in.clockPeriodStartC = clockPeriodStartC
	in.sentPeriodStart = false
	in.clockPeriodPostCloseC = clockPeriodPostCloseC
	in.sentPeriodPostClose = false
	in.pwd = pwd
	in.validator = v
	in.validatorHasAdded = false
	in.validatorHasWithdrawn = false
	in.history = ll.CreateGeneric[sch.Event]()

	in.data, err = v.Data()
	if err != nil {
		in.errorC <- err
	}

	go loopOpenReceipt(
		in.ctx,
		in.errorC,
		in.pipeline,
		in.receiptC,
		in.pwd.Payout,
		in.validator,
		in.clockPeriodStartC,
	)

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case id := <-eventHome.DeleteC:
			eventHome.Delete(id)
		case r := <-eventHome.ReqC:
			streamC := in.eventHome.Receive(r)
			for _, l := range in.history.Array() {
				select {
				case streamC <- l:
				default:
					log.Debug("should not have a stuffed stream channel")
				}
			}
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

func (in *internal) broadcast(event sch.Event) {
	in.history.Append(event)
	in.eventHome.Broadcast(event)
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
	in.broadcast(sch.CreateWithPayload(
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
