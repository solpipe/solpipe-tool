package payout

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

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
