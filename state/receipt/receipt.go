package receipt

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type Receipt struct {
	ctx            context.Context
	cancel         context.CancelFunc
	internalC      chan<- func(*internal)
	dataC          chan<- cba.Receipt
	Id             sgo.PublicKey
	updateReceiptC chan dssub.ResponseChannel[cba.Receipt]
}

type ReceiptWithData struct {
	Receipt Receipt
	Data    cba.Receipt
}

func CreateReceipt(ctx context.Context, d sub.ReceiptGroup) (e1 Receipt, err error) {
	internalC := make(chan func(*internal), 10)
	dataC := make(chan cba.Receipt, 10)
	//if d == nil {
	//	err = errors.New("no data")
	//	return
	//}
	log.Debugf("starting receipt with data=%+v", d)

	receiptHome := dssub.CreateSubHome[cba.Receipt]()
	updateReceiptC := receiptHome.ReqC
	ctxC, cancel := context.WithCancel(ctx)
	go loopInternal(ctxC, cancel, internalC, d.Data, dataC, receiptHome)
	e1 = Receipt{
		ctx:            ctxC,
		cancel:         cancel,
		internalC:      internalC,
		dataC:          dataC,
		Id:             d.Id,
		updateReceiptC: updateReceiptC,
	}

	return
}

func (e1 Receipt) Close() {
	e1.cancel()
}

func (e1 Receipt) OnClose() <-chan struct{} {
	return e1.ctx.Done()
}

// get updates on the stake
func (e1 Receipt) OnData() dssub.Subscription[cba.Receipt] {
	return dssub.SubscriptionRequest(e1.updateReceiptC, func(r cba.Receipt) bool { return true })
}

func (e1 Receipt) Data() (ans cba.Receipt, err error) {
	err = e1.ctx.Err()
	if err != nil {
		return
	}
	doneC := e1.ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan cba.Receipt, 1)
	e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no receipt")
		} else {
			errorC <- nil
			//log.Debugf("receipt sending data=%+v", in.data)
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
	select {
	case ans = <-ansC:
	case <-doneC:
		err = errors.New("canceled")
	}
	if err != nil {
		return
	}
	return
}
