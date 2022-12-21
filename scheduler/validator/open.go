package validator

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type receiptWithTransition struct {
	rwd           rpt.ReceiptWithData
	isStateChange bool
}

// loop until a receipt corresponding to our payout is received
func loopOpenReceipt(
	ctx context.Context,
	cancel context.CancelFunc,
	errorC chan<- error,
	rC chan<- receiptWithTransition,
	eventC chan<- sch.Event,
	payout pyt.Payout,
	v val.Validator,
	clockPeriodStartC <-chan bool,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	sub := v.OnReceipt()
	defer sub.Unsubscribe()
	payoutId := payout.Id
	rwd, present := v.ReceiptByPayoutId(payoutId)
	if present {
		select {
		case <-doneC:
			return
		case rC <- receiptWithTransition{
			rwd:           rwd,
			isStateChange: false,
		}:
		}
	} else {
		// no receipt has been created yet
		select {
		case <-doneC:
			return
		case eventC <- sch.CreateWithPayload(
			sch.TRIGGER_VALIDATOR_SET_PAYOUT,
			false,
			0,
			schpyt.Trigger{
				Context: ctx,
				Payout:  payout,
			},
		):
		}
	}
out:
	for {
		select {
		case <-doneC:
			break out
		case <-clockPeriodStartC:
			// period started before we received a receipt, so exit the loop
			select {
			case <-doneC:
			case errorC <- nil:
			}
			return
		case err = <-sub.ErrorC:
			break out
		case rwd = <-sub.StreamC:
			if rwd.Data.Payout.Equals(payoutId) {
				select {
				case <-doneC:
					break out
				case rC <- receiptWithTransition{
					rwd:           rwd,
					isStateChange: true,
				}:
				}
				break out
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
