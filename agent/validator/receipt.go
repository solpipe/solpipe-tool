package validator

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/slot"
)

func (in *internal) on_receipt(vri *validatorReceiptInfo) {
	vri.cancel()
}

type receiptInfo struct {
	ctx     context.Context
	errorC  chan<- error
	receipt rpt.Receipt
	data    *cba.Receipt
	slot    uint64
}

func loopReceipt(
	ctx context.Context,
	cancel context.CancelFunc,
	deleteC chan<- sgo.PublicKey,
	parentErrorC chan<- error,
	slotHome slot.SlotHome,
	rwd rpt.ReceiptWithData,
) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()

	ri := new(receiptInfo)
	ri.ctx = ctx
	ri.errorC = errorC
	ri.receipt = rwd.Receipt
	ri.data = &rwd.Data
	ri.slot = 0

	closeC := rwd.Receipt.OnClose()

	dataC := rwd.Receipt.OnData()
	defer dataC.Unsubscribe()

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
out:
	for {

		select {
		case <-closeC:
			break out
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case err = <-dataC.ErrorC:
			break out
		case x := <-dataC.StreamC:
			ri.on_data(x)
		case err = <-slotSub.ErrorC:
			break out
		case ri.slot = <-slotSub.StreamC:
			ri.check_close()
		}
	}

	if err != nil {
		select {
		case <-time.After(10 * time.Second):
		case parentErrorC <- err:
		}
	} else {
		select {
		case <-time.After(10 * time.Second):
		case parentErrorC <- err:
		}
	}
}

// figure out when to close the receipt
func (ri *receiptInfo) on_data(x cba.Receipt) {
	*ri.data = x
	ri.check_close()
}

func (ri *receiptInfo) check_close() {
	if ri.data.Limit < ri.slot {

	}
}

/*
// claim rent back, also claim revenue
func (pi *listenPipelineInternal) receipt_close(rwd rpt.ReceiptWithData) error {
	receiptId := rwd.Receipt.Id
	log.Debugf("attemping to close receipt id=%s (payout=%s)", receiptId.String(), rwd.Data.Payout.String())
	err := pi.scriptReceiptClose.SetTx(pi.config.Admin)
	if err != nil {
		return err
	}
	err = pi.scriptReceiptClose.ValidatorWithdrawReceipt(
		pi.router.Controller,
		rwd.Data.Payout,
		pi.pipeline,
		pi.validator,
		receiptId,
		pi.config.Admin,
	)
	if err != nil {
		return err
	}
	err = pi.scriptReceiptClose.FinishTx(true)
	if err != nil {
		return err
	}
	return nil
}

func (pi *listenPipelineInternal) receipt_delete(ri *receiptInfo) {
	log.Debugf("removing receipt for payout=%s", ri.pwd.Id.String())
	ri.cancel()

}
*/
