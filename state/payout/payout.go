package payout

import (
	"context"
	"errors"
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
)

const PAYOUT_CLOSE_DELAY = uint64(100)

type Payout struct {
	ctx              context.Context
	cancel           context.CancelFunc
	internalC        chan<- func(*internal)
	dataC            chan<- sub.PayoutWithData
	Id               sgo.PublicKey
	updateBidStatusC chan dssub.ResponseChannel[BidStatus]
	updatePayoutC    chan dssub.ResponseChannel[cba.Payout]
	updateReceiptC   chan dssub.ResponseChannel[rpt.ReceiptWithData]
}

/*
1. do append period
2. create: payout + bid + []receipt
3. do Validator Set Payout
4. update: payout (validator count++)
5. do Crank
6. update: payout + bid (final=true)
7. do ValidatorWithdraw
8. update: payout (validator count--); close: receipt
9. do CloseBids
10. update: payout; close:bid
11. do ClosePayouts
12. close: payout

*/

func CreatePayout(ctx context.Context, d sub.PayoutWithData) (e1 Payout, err error) {
	internalC := make(chan func(*internal), 10)
	dataC := make(chan sub.PayoutWithData, 10)
	bidStatusHome := dssub.CreateSubHome[BidStatus]()
	payoutHome := dssub.CreateSubHome[cba.Payout]()
	receiptHome := dssub.CreateSubHome[rpt.ReceiptWithData]()
	ctxC, cancel := context.WithCancel(ctx)
	go loopInternal(
		ctxC,
		internalC,
		d.Id,
		&d.Data,
		dataC,
		payoutHome,
		receiptHome,
		bidStatusHome,
	)
	e1 = Payout{
		ctx:              ctxC,
		cancel:           cancel,
		internalC:        internalC,
		dataC:            dataC,
		Id:               d.Id,
		updateBidStatusC: bidStatusHome.ReqC,
		updatePayoutC:    payoutHome.ReqC,
		updateReceiptC:   receiptHome.ReqC,
	}

	return
}

func (e1 Payout) Close() {
	log.Debugf("closing payout=%s", e1.Id.String())
	e1.cancel()
}

func (e1 Payout) OnClose() <-chan struct{} {
	return e1.ctx.Done()
}

func (e1 Payout) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	if err != nil {
		signalC <- err
	}
	return signalC

}

func (e1 Payout) OnData() dssub.Subscription[cba.Payout] {
	return dssub.SubscriptionRequest(e1.updatePayoutC, func(x cba.Payout) bool { return true })
}

func (e1 Payout) Data() (ans cba.Payout, err error) {
	err = e1.ctx.Err()
	if err != nil {
		return
	}
	doneC := e1.ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan cba.Payout, 1)
	e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no payout")
		} else {
			errorC <- nil
			ansC <- *in.data
		}
	}
	select {
	case err = <-errorC:
	case <-doneC:
		err = errors.New("canceled")
	}
	if err != nil {
		return
	}
	ans = <-ansC
	return
}

func (e1 Payout) Print() (content string, err error) {

	data, err := e1.Data()
	if err != nil {
		return
	}
	start := data.Period.Start
	length := data.Period.Length
	finish := start + length - 1
	content = fmt.Sprintf("payout id=%s; start=%d; length=%d; end=%d\n", data.Pipeline.String(), start, length, finish)
	return
}
