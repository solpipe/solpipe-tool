package validator

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

// this function only gets called once
func (in *internal) on_receipt(rt receiptWithTransition) {
	if in.cancelValidatorSetPayout != nil {
		in.cancelValidatorSetPayout()
		in.cancelValidatorSetPayout = nil
	}

	select {
	case <-in.ctx.Done():
	case in.eventC <- sch.Create(
		sch.EVENT_VALIDATOR_IS_ADDING,
		rt.isStateChange,
		0,
	):
	}
	in.receipt = rt.rwd.Receipt
	in.receiptData = &rt.rwd.Data
	var ctxC context.Context
	ctxC, in.cancelStake = context.WithCancel(in.ctx)
	go loopWaitStakeFinish(
		ctxC,
		in.cancelStake,
		rt.rwd,
		in.errorC,
		in.eventC,
		in.clockPeriodPostCloseC,
	)
}

// events: EVENT_STAKER_IS_ADDING, EVENT_STAKER_HAVE_WITHDRAWN(_EMPTY), EVENT_VALIDATOR_HAVE_WITHDRAWN
func loopWaitStakeFinish(
	ctx context.Context,
	cancel context.CancelFunc,
	rwd rpt.ReceiptWithData,
	errorC chan<- error,
	eventC chan<- sch.Event,
	clockPeriodPostCloseC <-chan bool,
) {

	doneC := ctx.Done()
	dataSub := rwd.Receipt.OnData()
	defer dataSub.Unsubscribe()

	data, err := rwd.Receipt.Data()
	if err != nil {
		errorC <- err
		return
	}
	stakersAddingStarted := false
	stakerCounter := data.StakerCounter

	isClockPostPeriodTransition := false

	if 0 < stakerCounter {
		stakersAddingStarted = true
		select {
		case <-doneC:
			return
		case eventC <- sch.Create(
			sch.EVENT_STAKER_IS_ADDING,
			false,
			0,
		):
		}
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case isClockPostPeriodTransition = <-clockPeriodPostCloseC:
			// validator withdraw without staker adding
			if !stakersAddingStarted && stakerCounter == 0 {
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(
					sch.EVENT_STAKER_HAVE_WITHDRAWN_EMPTY,
					isClockPostPeriodTransition,
					0,
				):
				}
				break out
			}
		case err = <-dataSub.ErrorC:
			// when the receipt account closes, we get a nil error
			break out
		case data = <-dataSub.StreamC:
			stakerCounter = data.StakerCounter
			if !stakersAddingStarted && 0 < stakerCounter {
				stakersAddingStarted = true
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(
					sch.EVENT_STAKER_IS_ADDING,
					true,
					0,
				):
				}
			}
			if stakersAddingStarted && stakerCounter == 0 {
				select {
				case <-doneC:
					return
				case eventC <- sch.Create(
					sch.EVENT_STAKER_HAVE_WITHDRAWN,
					true,
					0,
				):
				}
				break out
			}
		}
	}
	if err != nil {
		errorC <- err
	} else {
		// the receipt account has been closed
		select {
		case <-doneC:
			return
		case eventC <- sch.Create(
			sch.EVENT_VALIDATOR_HAVE_WITHDRAWN,
			true,
			0,
		):
		}
	}
}
