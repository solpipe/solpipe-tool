package payout

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx              context.Context
	id               sgo.PublicKey
	errorC           chan<- error
	closeSignalCList []chan<- error
	data             *cba.Payout
	bi               *bidInfo
	receipts         map[string]rpt.ReceiptWithData
	payoutHome       *dssub.SubHome[cba.Payout]
	receiptHome      *dssub.SubHome[rpt.ReceiptWithData]
	bidStatusHome    *dssub.SubHome[BidStatus]
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	id sgo.PublicKey,
	data *cba.Payout,
	dataC <-chan sub.PayoutWithData,
	payoutHome *dssub.SubHome[cba.Payout],
	receiptHome *dssub.SubHome[rpt.ReceiptWithData],
	bidStatusHome *dssub.SubHome[BidStatus],
) {
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.id = id
	in.data = data
	in.receipts = make(map[string]rpt.ReceiptWithData)
	in.payoutHome = payoutHome
	in.receiptHome = receiptHome
	in.bidStatusHome = bidStatusHome
	in.closeSignalCList = make([]chan<- error, 0)

	defer payoutHome.Close()
	defer receiptHome.Close()
	defer bidStatusHome.Close()

	err = in.init()
	if err != nil {
		errorC <- err
	}

out:
	for {
		select {
		case err = <-errorC:
			break out
		case <-doneC:
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
		case id := <-bidStatusHome.DeleteC:
			bidStatusHome.Delete(id)
		case d := <-bidStatusHome.ReqC:
			bidStatusHome.Receive(d)
		}
	}
	in.finish(err)
}

func (in *internal) init() error {
	err := in.init_bid()
	if err != nil {
		return err
	}
	return nil
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
