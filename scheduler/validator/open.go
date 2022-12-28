package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
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
	pipeline pipe.Pipeline,
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
		log.Debug("sending receipt back - 1")
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
		log.Debug("sending trigger validator set payout")
		select {
		case <-doneC:
			return
		case eventC <- sch.CreateWithPayload(
			sch.TRIGGER_VALIDATOR_SET_PAYOUT,
			false,
			0,
			&TriggerValidator{
				Context:  ctx,
				Payout:   payout,
				Pipeline: pipeline,
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
			log.Debug("lock period start")
			select {
			case <-doneC:
			case errorC <- nil:
			}
			return
		case err = <-sub.ErrorC:
			break out
		case rwd = <-sub.StreamC:
			log.Debugf("receipt=%s for payout=%s vs required payout=%s", rwd.Receipt.Id.String(), rwd.Data.Payout.String(), payoutId.String())
			if rwd.Data.Payout.Equals(payoutId) {
				log.Debug("sending receipt back - 2")
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

	log.Debug("exiting open receipt loop")
	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}