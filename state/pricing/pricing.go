package pricing

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Pricing struct {
	ctx       context.Context
	router    rtr.Router
	internalC chan<- func(*internal)
	bidder    sgo.PublicKey
	handler   ts.Handle
}

func Create(
	ctx context.Context,
	router rtr.Router,
	bidder sgo.PublicKey,
	handle ts.Handle,
) Pricing {
	internalC := make(chan func(*internal))
	go loopInternal(
		ctx,
		internalC,
		router,
		bidder,
		handle,
	)

	return Pricing{
		ctx:       ctx,
		router:    router,
		internalC: internalC,
		bidder:    bidder,
		handler:   handle,
	}
}

func (e1 Pricing) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	return signalC
}
