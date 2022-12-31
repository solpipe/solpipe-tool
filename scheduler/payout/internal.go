package payout

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type internal struct {
	ctx                       context.Context
	errorC                    chan<- error
	closeSignalCList          []chan<- error
	router                    rtr.Router
	data                      cba.Payout
	payout                    pyt.Payout
	eventHome                 *dssub.SubHome[sch.Event]
	hasStarted                bool
	hasFinished               bool
	isClockReadyToClose       bool
	bidIsFinal                bool
	bidHasClosed              bool
	validatorAddingHasStarted bool
	validatorHasWithdrawn     bool
	stakerAddingHasStarted    bool
	stakerAddingIsDone        bool
	cancelCrank               context.CancelFunc
	cancelCloseBid            context.CancelFunc
	cancelValidatorSetPayout  context.CancelFunc
	cancelValidatorWithdraw   context.CancelFunc
	keepPayoutOpen            bool
	history                   *ll.Generic[sch.Event]
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	eventHome *dssub.SubHome[sch.Event],
	router rtr.Router,
	pwd pipe.PayoutWithData,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 5)

	//dataSub := pwd.Payout.OnData()
	//defer dataSub.Unsubscribe()
	//bidSub := pwd.Payout.OnBidStatus()
	//defer bidSub.Unsubscribe()

	eventC := make(chan sch.Event)
	clockPeriodStartC := make(chan bool, 1)
	clockPeriodPostC := make(chan bool, 1)
	go loopClock(ctx, router.Controller, eventC, errorC, clockPeriodStartC, clockPeriodPostC, pwd.Data, pwd.Id)
	go loopPayoutEvent(ctx, pwd, eventC, errorC, clockPeriodStartC, clockPeriodPostC)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.router = router
	in.data = pwd.Data
	in.payout = pwd.Payout
	in.eventHome = eventHome
	in.history = ll.CreateGeneric[sch.Event]()

	in.hasStarted = false
	in.hasFinished = false
	in.isClockReadyToClose = false
	in.bidIsFinal = false
	in.bidHasClosed = false
	in.validatorAddingHasStarted = false
	in.validatorHasWithdrawn = false
	in.stakerAddingHasStarted = false
	in.stakerAddingIsDone = false
	in.keepPayoutOpen = true

	// run this first to make sure all subscriptions have the validator_set_payout trigger
	//in.run_validator_set_payout()

out:
	for in.keepPayoutOpen {
		select {
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		case event := <-eventC:
			err = in.on_event(event)
			if err != nil {
				break out
			}
		case id := <-eventHome.DeleteC:
			eventHome.Delete(id)
		case r := <-eventHome.ReqC:
			streamC := eventHome.Receive(r)
			for _, l := range in.history.Array() {
				select {
				case streamC <- l:
				default:
					log.Debug("should not have a stuffed stream channel")
				}
			}
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

func (in *internal) broadcast(event sch.Event) {
	in.history.Append(event)
	in.eventHome.Broadcast(event)
}
