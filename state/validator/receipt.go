package validator

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

// look up receipt by payout id
func (e1 Validator) ReceiptByPayoutId(payoutId sgo.PublicKey) (rwd rpt.ReceiptWithData, present bool) {
	present = false
	presentC := make(chan bool, 1)
	ansC := make(chan rpt.ReceiptWithData, 1)
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		return
	case e1.internalC <- func(in *internal) {
		x, p := in.receiptsByPayoutId[payoutId.String()]
		presentC <- p
		if p {
			ansC <- rpt.ReceiptWithData{
				Data: x.d, Receipt: x.r,
			}
		}
	}:
	}
	select {
	case <-doneC:
		return
	case present = <-presentC:
	}
	if !present {
		return
	}
	rwd = <-ansC
	return
}

func (e1 Validator) ReceiptById(id sgo.PublicKey) (rwd rpt.ReceiptWithData, present bool) {
	present = false
	presentC := make(chan bool, 1)
	ansC := make(chan rpt.ReceiptWithData, 1)
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		return
	case e1.internalC <- func(in *internal) {
		x, p := in.receiptsById[id.String()]
		presentC <- p
		if p {
			ansC <- rpt.ReceiptWithData{
				Data: x.d, Receipt: x.r,
			}
		}
	}:
	}
	select {
	case <-doneC:
		return
	case present = <-presentC:
	}
	if present {
		rwd = <-ansC
	}

	return
}

func (e1 Validator) AllReceipt() ([]rpt.ReceiptWithData, error) {

	ansC := make(chan []rpt.ReceiptWithData, 1)
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		l := make([]rpt.ReceiptWithData, len(in.receiptsByPayoutId))
		i := 0
		for _, r := range in.receiptsByPayoutId {
			l[i] = rpt.ReceiptWithData{Data: r.d, Receipt: r.r}
			i++
		}
		ansC <- l
	}:
	}
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case list := <-ansC:
		return list, nil
	}
}
