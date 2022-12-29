package payout

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
)

type payoutEventInfo struct {
	ctx                   context.Context
	errorC                chan<- error
	eventC                chan<- sch.Event
	pwd                   pipe.PayoutWithData
	bidsAreClosed         bool
	validatorHasAdded     bool
	validatorHasWithdrawn bool
	postDelayClockPast    bool
	ctxExtra              context.Context
	cancelExtra           context.CancelFunc
}

// clockPeriodStartC bool indicates if this is a state transition
func loopPayoutEvent(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	eventC chan<- sch.Event,
	errorC chan<- error,
	clockPeriodStartC <-chan bool,
	clockPeriodPostC <-chan bool,
) {

	var err error
	doneC := ctx.Done()

	clockPeriodPostCopyC := make(chan bool, 1)

	pei := new(payoutEventInfo)
	pei.pwd = pwd
	pei.ctx = ctx
	pei.errorC = errorC
	pei.eventC = eventC

	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()

	newData := pwd.Data

	zero := util.Zero()
	pei.ctxExtra, pei.cancelExtra = context.WithCancel(ctx)
	defer pei.cancelExtra()
	pei.bidsAreClosed = false
	log.Debugf("testing util.zero payout=%s bids=%s", pwd.Id.String(), pwd.Data.Bids.String())
	if newData.Bids.Equals(zero) {
		pei.on_bid_closed(false)
	} else {
		go loopBidSubIsFinal(pei.ctxExtra, pwd, errorC, eventC)
		go loopStakeStatus(pei.ctxExtra, pwd, errorC, eventC, clockPeriodPostCopyC)
	}
	pei.postDelayClockPast = false
	pei.validatorHasAdded = false
	if 0 < pwd.Data.ValidatorCount {
		pei.validatorHasAdded = true
	}
	pei.validatorHasWithdrawn = false

out:
	for !(pei.bidsAreClosed && pei.validatorHasWithdrawn) {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case isStateTransition := <-clockPeriodStartC:
			log.Debugf("payout=%s start time!", pei.pwd.Id.String())
			// period has started
			if !pei.validatorHasAdded {
				pei.on_validator_adding(isStateTransition)
				if newData.ValidatorCount == 0 {
					pei.on_validator_has_withdrawn(isStateTransition)
				}
			}
		case isStateTransition := <-clockPeriodPostC:
			clockPeriodPostCopyC <- isStateTransition
			log.Debugf("payout=%s post time!", pei.pwd.Id.String())
			if !pei.postDelayClockPast {
				pei.on_post_delay_close(isStateTransition)
				if !pei.validatorHasWithdrawn {
					pei.on_validator_has_withdrawn(isStateTransition)
				}
			}
		case newData = <-dataSub.StreamC:
			// check if BidClosed
			if !pei.bidsAreClosed && newData.Bids.Equals(zero) {
				pei.on_bid_closed(true)
			}

			// check if validators are signing up or cashing out
			if !pei.validatorHasAdded && 0 < newData.ValidatorCount {
				pei.on_validator_adding(true)
			}
			if pei.validatorHasAdded && !pei.validatorHasWithdrawn && newData.ValidatorCount == 0 {
				pei.on_validator_has_withdrawn(true)
			}
		}
	}
	log.Debugf("payout=%s exiting loop", pei.pwd.Id.String())

	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}

func (pei *payoutEventInfo) on_post_delay_close(isStateTransition bool) {
	pei.postDelayClockPast = true
}

func (pei *payoutEventInfo) on_validator_has_withdrawn(isStateTransition bool) {
	doneC := pei.ctx.Done()
	log.Debugf("payout=%s validator has withdrawn", pei.pwd.Id.String())
	pei.validatorHasWithdrawn = true
	select {
	case <-doneC:
		return
	case pei.eventC <- sch.Create(sch.EVENT_VALIDATOR_HAVE_WITHDRAWN, isStateTransition, 0):
	}
}

func (pei *payoutEventInfo) on_validator_adding(isStateTransition bool) {

	doneC := pei.ctx.Done()
	pei.validatorHasAdded = true
	select {
	case <-doneC:
		return
	case pei.eventC <- sch.Create(sch.EVENT_VALIDATOR_IS_ADDING, isStateTransition, 0):
	}
}
func (pei *payoutEventInfo) on_bid_closed(isStateTransition bool) {

	doneC := pei.ctx.Done()
	pei.bidsAreClosed = true
	select {
	case <-doneC:
		return
	case pei.eventC <- sch.Create(sch.EVENT_BID_CLOSED, isStateTransition, 0):
	}
	pei.cancelExtra()

}
