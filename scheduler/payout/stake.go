package payout

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

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
