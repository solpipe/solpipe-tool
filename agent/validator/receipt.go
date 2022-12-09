package validator

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/slot"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

// we just need to wait until the receipt
func (in *internal) on_receipt(vri *validatorReceiptInfo) {
	go loopReceipt(
		vri.ctx,
		vri.cancel,
		in.errorC,
		in.controller.SlotHome(),
		in.scriptWrapper,
		vri.rwd,
		in.config.Admin,
		in.controller,
		vri.pipeline,
		in.validator,
	)
}

type receiptInfo struct {
	ctx           context.Context
	errorC        chan<- error
	receipt       rpt.Receipt
	data          *cba.Receipt
	slot          uint64
	scriptWrapper spt.Wrapper
	close         bool
	admin         sgo.PrivateKey
}

func loopReceipt(
	ctx context.Context,
	cancel context.CancelFunc,
	parentErrorC chan<- error,
	slotHome slot.SlotHome,
	scriptWrapper spt.Wrapper,
	rwd rpt.ReceiptWithData,
	admin sgo.PrivateKey,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	validator val.Validator,
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
	ri.scriptWrapper = scriptWrapper
	ri.close = false
	ri.admin = admin

	closeC := rwd.Receipt.OnClose()

	dataC := rwd.Receipt.OnData()
	defer dataC.Unsubscribe()

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
out:
	for !ri.close {
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
		parentErrorC <- err
		return
	}
	err = ri.do_close(controller, pipeline, validator)
	if err != nil {
		parentErrorC <- err
	}
}

// figure out when to close the receipt
func (ri *receiptInfo) on_data(x cba.Receipt) {
	*ri.data = x
	ri.check_close()
}

func (ri *receiptInfo) check_close() {
	if ri.data.Limit < ri.slot {
		ri.close = true
	}
}

func (ri *receiptInfo) do_close(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	validator val.Validator,
) error {
	return ri.scriptWrapper.Send(ri.ctx, 10, 30*time.Second, func(script *spt.Script) error {
		err2 := script.SetTx(ri.admin)
		if err2 != nil {
			return err2
		}
		err2 = script.ValidatorWithdrawReceipt(
			controller,
			ri.data.Payout,
			pipeline,
			validator,
			ri.receipt.Id,
			ri.admin,
		)
		if err2 != nil {
			return err2
		}
		err2 = script.FinishTx(true)
		if err2 != nil {
			return err2
		}
		return err2
	})
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
