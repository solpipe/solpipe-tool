package router

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
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

func (in *internal) on_receipt(d sub.ReceiptGroup) error {
	id := d.Id
	if !d.IsOpen {
		y, present := in.l_receipt.byId[id.String()]
		if present {
			y.Close()
			delete(in.l_receipt.byId, id.String())
		}
		return nil
	}
	data := d.Data
	var err error

	newlyCreated := false

	r, present := in.l_receipt.byId[id.String()]
	if !present {
		r, err = rpt.CreateReceipt(in.ctx, d)
		if err != nil {
			return err
		}
		in.l_receipt.byId[r.Id.String()] = r

		newlyCreated = true
	} else {
		r.Update(data)
	}

	{
		x, present := in.l_receipt.stakerWithNoReceipt[r.Id.String()]
		if present {
			for _, s := range x {
				r.UpdateStaker(s.obj, s.mgr)
			}
			delete(in.l_receipt.stakerWithNoReceipt, r.Id.String())
		}
	}

	if newlyCreated {
		in.oa.receipt.Broadcast(r)
		go loopDelete(in.ctx, r.OnClose(), in.reqClose.receiptCloseC, r.Id, in.ws)

		pwd, present := in.l_payout.byId[data.Payout.String()]
		if present {
			pwd.p.UpdateReceipt(r)
		} else {
			x, present := in.l_payout.receiptWithNoPayout[data.Payout.String()]
			if !present {
				x = make(map[string]rpt.Receipt)
				in.l_payout.receiptWithNoPayout[data.Payout.String()] = x
			}
			_, present = x[r.Id.String()]
			if !present {
				x[r.Id.String()] = r
			}
		}
		vwd, present := in.l_validator.byId[data.Validator.String()]
		if present {
			vwd.v.UpdateReceipt(r)
		} else {
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
