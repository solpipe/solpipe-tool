package payout

import (
	"errors"

	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

// get all receipts
func (e1 Payout) Receipt() ([]rpt.ReceiptWithData, error) {
	doneC := e1.ctx.Done()
	err := e1.ctx.Err()
	if err != nil {
		return nil, err
	}
	countC := make(chan int, 1)
	ansC := make(chan rpt.ReceiptWithData, 1)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		countC <- len(in.receipts)
		for _, r := range in.receipts {
			ansC <- r
		}
	}:
	}
	if err != nil {
		return nil, err
	}
	var n int
	select {
	case <-doneC:
		err = errors.New("canceled")
	case n = <-countC:
	}
	if err != nil {
		return nil, err
	}
	ans := make([]rpt.ReceiptWithData, n)
	for i := 0; i < n; i++ {
		ans[i] = <-ansC
	}
	return ans, nil
}
