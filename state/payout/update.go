package payout

import (
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/util"
)

func (e1 Payout) Update(pwd sub.PayoutWithData) {
	select {
	case <-e1.ctx.Done():
	case e1.dataC <- pwd:
	}
}

// If you get IsOpen=true, then router will call e1.Close() for us. Do not close Payout here.
func (in *internal) on_data(pwd sub.PayoutWithData) {
	log.Debugf("zero? bid=%s", pwd.Data.Bids.String())
	if pwd.Data.Bids.Equals(util.Zero()) {
		in.close_bid_list()
	}
	in.payoutHome.Broadcast(pwd.Data)
	in.data = &pwd.Data
}

func (e1 Payout) UpdateBidList(bl cba.BidList) {
	select {
	case <-e1.ctx.Done():
	case e1.internalC <- func(in *internal) {
		in.on_bid_list(bl)
	}:
	}
}

// do not worry about deleting receipts;  we worry about deleting the payout.
func (e1 Payout) UpdateReceipt(r rpt.Receipt) {
	doneC := e1.ctx.Done()
	data, err := r.Data()
	if err != nil {
		log.Error(err)
		return
	}
	presentC := make(chan bool, 1)
	select {
	case <-doneC:
	case e1.internalC <- func(in *internal) {
		presentC <- in.on_receipt(r, data)
	}:
	}
	select {
	case <-doneC:
	case setDelete := <-presentC:
		if setDelete {
			go e1.receipt_delete(r)
		}
	}
}

func (e1 Payout) receipt_delete(r rpt.Receipt) {
	id := r.Id
	closeC := r.OnClose()
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		return
	case <-closeC:
	}
	select {
	case <-doneC:
	case e1.internalC <- func(in *internal) {
		delete(in.receipts, id.String())
	}:
	}
}

func (in *internal) on_receipt(
	r rpt.Receipt,
	data cba.Receipt,
) bool {
	_, present := in.receipts[data.Validator.String()]
	if present {
		return true
	}
	rwd := rpt.ReceiptWithData{Receipt: r, Data: data}
	in.receipts[data.Validator.String()] = rwd
	in.receiptHome.Broadcast(rwd)
	return false
}

// if validator=util.Zero(), then all receipts will be returned
func (e1 Payout) OnReceipt(validator sgo.PublicKey) dssub.Subscription[rpt.ReceiptWithData] {
	cb := func(x rpt.ReceiptWithData) bool {
		if x.Data.Validator.Equals(validator) {
			return true
		} else {
			return false
		}
	}
	if validator.Equals(util.Zero()) {
		cb = func(x rpt.ReceiptWithData) bool {
			return true
		}
	}
	return dssub.SubscriptionRequest(e1.updateReceiptC, cb)
}
