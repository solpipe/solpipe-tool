package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
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
	errorC chan<- error,
	pipeline pipe.Pipeline,
	receiptC chan<- receiptWithTransition,
	payout pyt.Payout,
	v val.Validator,
	clockPeriodStartC <-chan bool,
) {
	var err error
	doneC := ctx.Done()
	sub := v.OnReceipt()
	defer sub.Unsubscribe()
	payoutId := payout.Id

	rwd, present := v.ReceiptByPayoutId(payoutId)
	if present {
		log.Debugf("sending receipt back - 1 payout=%s", payout.Id.String())
		select {
		case <-doneC:
			return
		case receiptC <- receiptWithTransition{
			rwd:           rwd,
			isStateChange: false,
		}:
		}
		return
	}
	// we used to send a trigger validator_set_payout here, but decided to send it instead from the payout schedule
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case rwd = <-sub.StreamC:
			log.Debugf("receipt=%s for payout=%s vs required payout=%s", rwd.Receipt.Id.String(), rwd.Data.Payout.String(), payoutId.String())
			if rwd.Data.Payout.Equals(payoutId) {
				log.Debugf("sending receipt back - 2 payout=%s", payoutId.String())
				select {
				case <-doneC:
					break out
				case receiptC <- receiptWithTransition{
					rwd:           rwd,
					isStateChange: true,
				}:
				}
				break out
			}
		case <-clockPeriodStartC:
			// period started before we received a receipt, so exit the loop
			log.Debugf("lock period start payout=%s", payoutId.String())
			break out

		}
	}

	log.Debugf("exiting open receipt loop; payout=%s", payoutId.String())
	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}
