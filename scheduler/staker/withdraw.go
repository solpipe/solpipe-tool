package staker

import (
	"context"

	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
)

// TRIGGER_STAKER_WITHDRAW
func loopWithdraw(
	ctx context.Context,
	cancel context.CancelFunc,
	ps sch.Schedule,
	pwd pipe.PayoutWithData,
	rwd rpt.ReceiptWithData,
	s skr.Staker,
	srg sub.StakerReceiptGroup,
	errorC chan<- error,
	eventC chan<- sch.Event,
) {

	var err error
	doneC := ctx.Done()
	receiptSub := s.OnReceipt(rwd.Receipt.Id)
	defer receiptSub.Unsubscribe()

	eventSub := ps.OnEvent()
	defer eventSub.Unsubscribe()

out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-receiptSub.ErrorC:
			break out
		case srg = <-receiptSub.StreamC:
			if !srg.IsOpen {
				select {
				case <-doneC:
				case eventC <- sch.Create(
					schpyt.EVENT_STAKER_HAVE_WITHDRAWN,
					true,
					0,
				):
				}
				return
			}
		case err = <-eventSub.ErrorC:
			break out
		case event := <-eventSub.StreamC:
			switch event.Type {
			case schpyt.EVENT_FINISH:
				select {
				case <-doneC:
					break out
				case eventC <- sch.CreateWithPayload(
					schpyt.TRIGGER_STAKER_WITHDRAW,
					true,
					0,
					&TriggerReceipt{
						TriggerStaker: TriggerStaker{
							Context: ctx,
							Staker:  s,
							Payout:  pwd.Payout,
							Receipt: rwd.Receipt,
						},
						StakerReceipt: srg,
					},
				):
				}
				break out
			default:
			}
		}
	}

	if err != nil {
		errorC <- err
	}
}

type TriggerReceipt struct {
	TriggerStaker
	StakerReceipt sub.StakerReceiptGroup
}
