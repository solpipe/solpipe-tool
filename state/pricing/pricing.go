package pricing

import (
	"context"
	"errors"

	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Pricing struct {
	ctx       context.Context
	router    rtr.Router
	internalC chan<- func(*internal)
}

func Create(
	ctx context.Context,
	router rtr.Router,
) Pricing {
	internalC := make(chan func(*internal))
	go loopInternal(
		ctx,
		internalC,
		router,
	)

	return Pricing{
		ctx:       ctx,
		router:    router,
		internalC: internalC,
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
