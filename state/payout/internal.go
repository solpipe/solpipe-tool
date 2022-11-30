package payout

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	data             *cba.Payout
	receipts         map[string]rpt.ReceiptWithData
	payoutHome       *sub2.SubHome[cba.Payout]
	receiptHome      *sub2.SubHome[rpt.ReceiptWithData]
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	data *cba.Payout,
	dataC <-chan sub.PayoutWithData,
	payoutHome *sub2.SubHome[cba.Payout],
	receiptHome *sub2.SubHome[rpt.ReceiptWithData],
) {
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.data = data
	in.receipts = make(map[string]rpt.ReceiptWithData)
	in.payoutHome = payoutHome
	in.receiptHome = receiptHome
	in.closeSignalCList = make([]chan<- error, 0)

	defer payoutHome.Close()
	defer receiptHome.Close()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case d := <-dataC:
			in.on_data(d)
		case id := <-payoutHome.DeleteC:
			payoutHome.Delete(id)
		case d := <-payoutHome.ReqC:
			payoutHome.Receive(d)
		case id := <-receiptHome.DeleteC:
			receiptHome.Delete(id)
		case d := <-receiptHome.ReqC:
			receiptHome.Receive(d)
		}
	}
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

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
