package validator

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/slot"
)

// listen for a receipt.
// wait til stakers claim their payments, then delete the receipt account and claim validator payout
func loopReceipt(
	ctx context.Context,
	errorC chan<- error,
	setValidatorOnPayoutC chan<- sgo.PublicKey,
	slotHome slot.SlotHome,
	period cba.Period,
	item pipe.PayoutWithData,
	validatorId sgo.PublicKey,
) {
	p := item.Payout
	fetchErrorC := make(chan error, 1)
	rwdC := make(chan rpt.ReceiptWithData, 1)
	go loopGetReceiptWithData(
		ctx,
		validatorId,
		p,
		fetchErrorC,
		rwdC,
	)
	doneC := ctx.Done()

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	var err error

	slot := uint64(0)
	var rwd rpt.ReceiptWithData
	hasRwd := false
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-fetchErrorC:
			break out
		case rwd = <-rwdC:
			hasRwd = true
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
		}
	}

	if err != nil {
		errorC <- err
	}
	if !hasRwd {
		return
	}

	stakeStart := false

	stakeCount := rwd.Data.StakerCounter

	end := period.Start + period.Length

out2:
	for !stakeStart || (stakeStart && 0 < stakeCount) || slot < end+100 {
		select {
		case <-doneC:
			break out2
		case err = <-fetchErrorC:
			break out2
		case rwd = <-rwdC:
			if !stakeStart && 0 < rwd.Data.StakerCounter {
				stakeStart = true
			}
			stakeCount = rwd.Data.StakerCounter
		case err = <-slotSub.ErrorC:
			break out2
		case slot = <-slotSub.StreamC:
		}
	}
	// TODO: validator claim payment
	if err != nil {
		log.Debug(err)
	}
}

// find out if we need to create a receipt, then wait for a receipt to be created
func loopGetReceiptWithData(
	ctx context.Context,
	validatorId sgo.PublicKey,
	p pyt.Payout,
	errorC chan<- error,
	rwdC chan<- rpt.ReceiptWithData,
) {
	doneC := ctx.Done()
	hasRwd := false
	var rwd rpt.ReceiptWithData
	list, err := p.Receipt()
	if err == nil {
		for _, z := range list {
			if z.Data.Validator.Equals(validatorId) {
				hasRwd = true
				rwd = z
			}
		}
	}
	if hasRwd {
		errorC <- nil
		rwdC <- rwd
		return
	}

	receiptSub := p.OnReceipt(validatorId)
	defer receiptSub.Unsubscribe()

	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-receiptSub.ErrorC:
	case rwd = <-receiptSub.StreamC:
	}

	if err != nil {
		errorC <- err
	} else {
		errorC <- nil
		rwdC <- rwd
	}
}
