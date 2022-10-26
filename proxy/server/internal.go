package server

import (
	"context"

	rtr "github.com/solpipe/solpipe-tool/state/router"
	sgo "github.com/SolmateDev/solana-go"
)

type internal struct {
	ctx              context.Context
	closeSignalCList []chan<- error
	errorC           chan<- error
	router           rtr.Router
	updateReceipt    map[string]*receiptWithArgs // txhash->receipt
	deleteReceiptC   chan<- string
	updateStream     map[string]*streamInfo // sender -> stream
	deleteStreamC    chan<- string
}

type streamInfo struct {
	errorC                     chan<- error
	sendReceiptUpdateToSenderC chan<- *sgo.Transaction
}

type receiptWithArgs struct {
	sender                     sgo.PublicKey
	sendReceiptUpdateToSenderC chan<- *sgo.Transaction
	pair                       *ReceiptPair
	tx                         *sgo.Transaction
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	router rtr.Router,
) {
	defer cancel()
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	deleteReceiptC := make(chan string, 1)
	deleteStreamC := make(chan string, 1)
	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.deleteReceiptC = deleteReceiptC
	in.deleteStreamC = deleteStreamC
	var err error
out:
	for {
		select {
		case id := <-deleteReceiptC:
			delete(in.updateReceipt, id)
		case id := <-deleteStreamC:
			delete(in.updateStream, id)
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		}
	}
	in.finish(err)
}
func (in *internal) finish(err error) {
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
