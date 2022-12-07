package validator

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/slot"
)

type receiptInfo struct {
	pwd       pipe.PayoutWithData
	cancel    context.CancelFunc
	receiptId sgo.PublicKey
}

func (ri *receiptInfo) Id() sgo.PublicKey {
	return ri.pwd.Id
}

const RECEIPT_END_DELAY uint64 = 100

// listen for a receipt.
// wait til stakers claim their payments, then delete the receipt account and claim validator payout
func loopReceipt(
	ctx context.Context,
	cancel context.CancelFunc,
	errorC chan<- error,
	slotHome slot.SlotHome,
	period cba.Period,
	item pipe.PayoutWithData,
	validatorId sgo.PublicKey,
) {
	defer cancel()
	p := item.Payout
	fetchErrorC := make(chan error, 1)
	rwdC := make(chan rpt.ReceiptWithData, 1)
	receiptSub := p.OnReceipt(validatorId)
	defer receiptSub.Unsubscribe()
	ctxC, cancelC := context.WithCancel(ctx)
	defer cancelC()
	go loopGetReceiptWithData(
		ctxC,
		validatorId,
		p,
		fetchErrorC,
		rwdC,
	)
	doneC := ctx.Done()

	var err error
	var rwd rpt.ReceiptWithData
	hasRwd := false
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-fetchErrorC:
			break out
		case err = <-receiptSub.ErrorC:
			break out
		case rwd = <-receiptSub.StreamC:
			hasRwd = true
			break out
		case rwd = <-rwdC:
			hasRwd = true
			break out
		}
	}

	if err != nil {
		errorC <- err
		return
	}
	if !hasRwd {
		return
	}

	stakeStart := false
	stakeCount := rwd.Data.StakerCounter
	finish := period.Start + period.Length

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	slot := uint64(0)

	// 1. the stakeCount starts at 0
	// 2. the stakeCount goes up as each staker reports their stake
	// 3. the stakeCount goes down as each staker claims revenue
	// 4. the validator can claim revenue, thereby closing this receipt account
out2:
	for !stakeStart || (stakeStart && 0 < stakeCount) || slot < finish+RECEIPT_END_DELAY {
		select {
		case <-doneC:
			break out2
		case err = <-fetchErrorC:
			break out2
		case err = <-receiptSub.ErrorC:
			break out2
		case rwd = <-receiptSub.StreamC:
			hasRwd = true
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
		return
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
		// use selects to make sure we do not block
		select {
		case <-doneC:
		case errorC <- nil:
			select {
			case <-doneC:
			case rwdC <- rwd:
			}
		}
	}
}

// claim rent back, also claim revenue
func (pi *listenPeriodInternal) receipt_close(ri *receiptInfo) error {
	err := pi.script.SetTx(pi.config.Admin)
	if err != nil {
		return err
	}
	err = pi.script.ValidatorWithdrawReceipt(
		pi.router.Controller,
		ri.pwd.Id,
		pi.pipeline,
		pi.validator,
		pi.config.Admin,
		ri.receiptId,
	)
	if err != nil {
		return err
	}
	err = pi.script.FinishTx(true)
	if err != nil {
		return err
	}
	return nil
}

func (pi *listenPeriodInternal) receipt_delete(ri *receiptInfo) {
	log.Debugf("removing receipt for payout=%s", ri.pwd.Id.String())
	ri.cancel()

}
