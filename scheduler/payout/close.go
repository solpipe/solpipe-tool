package payout

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
)

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

	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()

	newData := pwd.Data

	zero := util.Zero()
	ctxExtra, cancelExtra := context.WithCancel(ctx)
	defer cancelExtra()
	bidsAreClosed := false
	if newData.Bids.Equals(zero) {
		bidsAreClosed = true
		select {
		case <-doneC:
			return
		case eventC <- sch.Create(sch.EVENT_BID_CLOSED, false, 0):
		}
		cancelExtra()
	} else {
		go loopBidSubIsFinal(ctxExtra, pwd, errorC, eventC)
		go loopStakeStatus(ctxExtra, pwd, errorC, eventC, clockPeriodPostC)
	}

	validatorHasAdded := false
	if 0 < pwd.Data.ValidatorCount {
		validatorHasAdded = true
	}
	validatorHasWithdrawn := false

out:
	for !bidsAreClosed && !validatorHasAdded && !validatorHasWithdrawn {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case isStateTransition := <-clockPeriodStartC:
			// period has started
			if !validatorHasAdded {
				validatorHasAdded = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_VALIDATOR_IS_ADDING, isStateTransition, 0):
				}
				if newData.ValidatorCount == 0 {
					validatorHasWithdrawn = true
					select {
					case <-doneC:
						return
					case eventC <- sch.Create(sch.EVENT_VALIDATOR_HAVE_WITHDRAWN, isStateTransition, 0):
					}
				}
			}
		case newData = <-dataSub.StreamC:
			// check if BidClosed
			if !bidsAreClosed && newData.Bids.Equals(zero) {
				bidsAreClosed = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_BID_CLOSED, true, 0):
				}
				cancelExtra()
			}

			// check if validators are signing up or cashing out
			if !validatorHasAdded && 0 < newData.ValidatorCount {
				validatorHasAdded = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_VALIDATOR_IS_ADDING, true, 0):
				}
			}
			if validatorHasAdded && !validatorHasWithdrawn && newData.ValidatorCount == 0 {
				validatorHasWithdrawn = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_VALIDATOR_HAVE_WITHDRAWN, true, 0):
				}
			}
		}
	}

	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}

func loopBidSubIsFinal(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	errorC chan<- error,
	eventC chan<- sch.Event,
) {
	var err error
	doneC := ctx.Done()
	bidSub := pwd.Payout.OnBidStatus()
	defer bidSub.Unsubscribe()

	bsIsFinal := false
	bs, err := pwd.Payout.BidStatus()
	if err != nil {
		errorC <- err
		return
	} else if bs.IsFinal {
		bsIsFinal = true
		select {
		case <-doneC:
			return
		case eventC <- sch.Create(sch.EVENT_BID_FINAL, false, 0):
		}
	}
out:
	for !bsIsFinal {
		select {
		case <-doneC:
			break out
		case err = <-bidSub.ErrorC:
			break out
		case bs = <-bidSub.StreamC:
			if !bsIsFinal && bs.IsFinal {
				bsIsFinal = true
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(sch.EVENT_BID_FINAL, true, 0):
				}

			}
		}
	}
	if err != nil {
		errorC <- err
		return
	}
}

func loopStakeStatus(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	errorC chan<- error,
	eventC chan<- sch.Event,
	clockPostDelayC <-chan bool, // 100 slot delay is over
) {
	var err error
	doneC := ctx.Done()
	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()

	data := pwd.Data

	hasStakerAdded := false
	isStakerAddedTransition := false
	hasStakerRemoved := false
	isPostPastTransition := false

	if 0 < data.StakerCount {
		hasStakerAdded = true
		isStakerAddedTransition = false
		select {
		case <-doneC:
			return
		case eventC <- sch.Create(sch.EVENT_STAKER_IS_ADDING, isStakerAddedTransition, 0):
		}
	}
	// we cannot test if the staker count has gone up and down again without monitoring for changes

out:
	for !hasStakerAdded && !hasStakerRemoved {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case data = <-dataSub.StreamC:
			if !hasStakerAdded && 0 < data.StakerCount {
				hasStakerAdded = true
				isStakerAddedTransition = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_STAKER_IS_ADDING, isStakerAddedTransition, 0):
				}
			}
			if hasStakerAdded && !hasStakerRemoved && data.StakerCount == 0 {
				hasStakerRemoved = true
				isStakerAddedTransition = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_STAKER_HAVE_WITHDRAWN, isStakerAddedTransition, 0):
				}
				break out
			}
		case isPostPastTransition = <-clockPostDelayC:
			if !hasStakerAdded && data.StakerCount == 0 {
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(sch.EVENT_STAKER_HAVE_WITHDRAWN, isPostPastTransition, 0):
				}
				break out
			}
		}
	}
	if err != nil {
		errorC <- err
		return
	}
}
