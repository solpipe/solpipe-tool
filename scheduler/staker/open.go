package staker

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	skr "github.com/solpipe/solpipe-tool/state/staker"
)

// TRIGGER_STAKER_ADD
func loopOpenReceipt(
	ctx context.Context,
	cancel context.CancelFunc,
	pwd pipe.PayoutWithData,
	rwd rpt.ReceiptWithData,
	s skr.Staker,
	errorC chan<- error,
	eventC chan<- sch.Event,
	srgtC chan<- srgWithTransition,
) {
	var err error
	doneC := ctx.Done()
	receiptSub := s.OnReceipt(rwd.Receipt.Id)
	defer receiptSub.Unsubscribe()

	// wait for staker receipt
	{
		list, err := s.AllReceipts()
		if err != nil {
			errorC <- err
		}
		for _, rg := range list {
			if rg.Data.Receipt.Equals(rwd.Receipt.Id) {
				select {
				case <-doneC:
				case srgtC <- srgWithTransition{
					rg:            rg,
					isStateChange: false,
				}:
				}
				return
			}
		}
	}
	select {
	case <-doneC:
		return
	case eventC <- sch.CreateWithPayload(
		schpyt.TRIGGER_STAKER_ADD,
		true,
		0,
		&TriggerStaker{
			Context: ctx,
			Payout:  pwd.Payout,
			Receipt: rwd.Receipt,
			Staker:  s,
		},
	):
	}

out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-receiptSub.ErrorC:
			break out
		case rg := <-receiptSub.StreamC:
			select {
			case <-doneC:
			case srgtC <- srgWithTransition{
				rg:            rg,
				isStateChange: true,
			}:
			}
			break out
		}
	}
	if err != nil {
		errorC <- err
	}
}
