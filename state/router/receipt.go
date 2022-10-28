package router

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type lookUpReceipt struct {
	byId                map[string]rpt.Receipt
	stakerWithNoReceipt map[string]map[string]stakerManagerExtra // receipt id->staker id->staker
}

func createLookupReceipt() *lookUpReceipt {
	return &lookUpReceipt{
		byId:                make(map[string]rpt.Receipt),
		stakerWithNoReceipt: make(map[string]map[string]stakerManagerExtra),
	}
}

func (in *internal) lookup_add_receipt(r rpt.Receipt, data cba.Receipt) {
	in.l_receipt.byId[r.Id.String()] = r

	// add receipt to validator
	{
		x, present := in.l_validator.receiptWithNoValidator[data.Validator.String()]
		if !present {
			x = make(map[string]rpt.Receipt)
			in.l_validator.receiptWithNoValidator[data.Validator.String()] = x
		}
		_, present = x[r.Id.String()]
		if !present {
			x[r.Id.String()] = r
		}
	}

	// TODO: add receipt to payout
}

func (in *internal) on_receipt(obj sub.ReceiptGroup) error {
	id := obj.Id
	if !obj.IsOpen {
		y, present := in.l_receipt.byId[id.String()]
		if present {
			y.Close()
			delete(in.l_receipt.byId, id.String())
		}
		return nil
	}
	data := obj.Data
	var err error

	newlyCreated := false

	r, present := in.l_receipt.byId[id.String()]
	if !present {
		r, err = rpt.CreateReceipt(in.ctx, obj)
		if err != nil {
			return err
		}
		in.lookup_add_receipt(r, data)
		newlyCreated = true
	} else {
		r.Update(data)
	}

	x, present := in.l_receipt.stakerWithNoReceipt[r.Id.String()]
	if present {
		for _, s := range x {
			r.UpdateStaker(s.obj, s.mgr)
		}
		delete(in.l_receipt.stakerWithNoReceipt, r.Id.String())
	}

	if newlyCreated {
		in.oa.receipt.Broadcast(r)
		go loopDelete(in.ctx, r.OnClose(), in.reqClose.receiptCloseC, r.Id, in.ws)
	}

	return nil
}

func (e1 Router) ReceiptById(id sgo.PublicKey) (rpt.Receipt, error) {
	errorC := make(chan error, 1)
	ansC := make(chan rpt.Receipt, 1)
	e1.internalC <- func(in *internal) {
		r, present := in.l_receipt.byId[id.String()]
		if !present {
			errorC <- errors.New("no receipt")
		}
		errorC <- nil
		ansC <- r
	}
	var err error
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return rpt.Receipt{}, err
	}
	return <-ansC, nil
}
