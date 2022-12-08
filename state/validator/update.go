package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

func (e1 Validator) Update(data cba.ValidatorManager) {
	e1.internalC <- func(in *internal) {
		in.on_data(data)
	}
}

// get alerted when the pipeline data has been changed
// the payout is not changed until UpdatePayout is called
func (in *internal) on_data(data cba.ValidatorManager) {
	in.data = &data
	//in.validatorHome.Broadcast(data)

	in.validatorHome.Broadcast(data)
}

func (e1 Validator) UpdateReceipt(r rpt.Receipt) {
	doneC := e1.ctx.Done()
	d, err := r.Data()
	if err != nil {
		log.Error(err)
		return
	}

	select {
	case <-doneC:
	case e1.internalC <- func(in *internal) {
		in.on_receipt(r, d)
	}:
	}

}

type receiptHolder struct {
	cancel context.CancelFunc
	r      rpt.Receipt
	d      cba.Receipt
}

func (in *internal) on_receipt(r rpt.Receipt, d cba.Receipt) {
	_, present := in.receipts[d.Payout.String()]
	if present {
		log.Debugf("receipt duplicate in validator: id=%s", r.Id.String())
		log.Debugf("%+v", d)
		return
	}

	ctx, cancel := context.WithCancel(in.ctx)
	in.receipts[d.Payout.String()] = &receiptHolder{
		cancel: cancel,
		r:      r,
		d:      d,
	}
	ri := new(receiptInternal)
	ri.ctx = ctx
	ri.errorC = in.errorC
	ri.updateC = in.updateReceiptC

	go ri.loop(r)

	in.receiptHome.Broadcast(rpt.ReceiptWithData{
		Receipt: r, Data: d,
	})
}

type receiptInternal struct {
	ctx     context.Context
	errorC  chan<- error
	updateC chan<- cba.Receipt
}

func (ri *receiptInternal) loop(r rpt.Receipt) {
	var err error
	doneC := ri.ctx.Done()
	sub := r.OnData()
	defer sub.Unsubscribe()
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case d := <-sub.StreamC:
			ri.updateC <- d
		}
	}

	if err != nil {
		ri.errorC <- err
	}
}

func (in *internal) on_receipt_update(d cba.Receipt) {
	rh, present := in.receipts[d.Payout.String()]
	if !present {
		log.Errorf("missing receipt payout=%s", d.Payout.String())
		return
	}
	rh.d = d
}
