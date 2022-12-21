package payout

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
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
	validatorAddingIsDone     bool
	stakerAddingHasStarted    bool
	stakerAddingIsDone        bool
	cancelCrank               context.CancelFunc
	cancelCloseBid            context.CancelFunc
	cancelValidatorSetPayout  context.CancelFunc
	cancelValidatorWithdraw   context.CancelFunc
	cancelStakerWithdraw      context.CancelFunc
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

	slotHome := router.Controller.SlotHome()

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	//dataSub := pwd.Payout.OnData()
	//defer dataSub.Unsubscribe()
	//bidSub := pwd.Payout.OnBidStatus()
	//defer bidSub.Unsubscribe()

	eventC := make(chan sch.Event)
	clockPeriodStartC := make(chan bool, 1)
	clockPeriodPostC := make(chan bool, 1)
	go loopClock(ctx, router.Controller, eventC, errorC, clockPeriodStartC, clockPeriodPostC, pwd.Data)
	go loopPayoutEvent(ctx, pwd, eventC, errorC, clockPeriodStartC, clockPeriodPostC)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.router = router
	in.data = pwd.Data
	in.payout = pwd.Payout
	in.eventHome = eventHome

	in.hasStarted = false
	in.hasFinished = false
	in.isClockReadyToClose = false
	in.bidIsFinal = false
	in.bidHasClosed = false
	in.validatorAddingHasStarted = false
	in.validatorAddingIsDone = false
	in.stakerAddingHasStarted = false
	in.stakerAddingIsDone = false

	var isTrans string

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		case event := <-eventC:
			if event.IsStateChange {
				isTrans = "change"
			} else {
				isTrans = "static"
			}
			log.Debugf("event payout=%s  type=%s  isTransition=%s", pwd.Id.String(), event.Type, isTrans)
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
				err = errors.New("unknown event")
				break out
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
	if in.cancelCloseBid != nil {
		in.cancelCloseBid()
	}
	if in.cancelCrank != nil {
		in.cancelCrank()
	}
	if in.cancelValidatorSetPayout != nil {
		in.cancelValidatorSetPayout()
	}
	if in.cancelValidatorWithdraw != nil {
		in.cancelValidatorWithdraw()
	}

	if in.cancelStakerWithdraw != nil {
		in.cancelStakerWithdraw()
	}
}
