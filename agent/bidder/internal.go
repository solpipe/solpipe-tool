package bidder2

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	router rtr.Router,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for _, signalC := range in.closeSignalCList {
		signalC <- err
	}
}

func (e1 Agent) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	select {
	case <-e1.ctx.Done():
		signalC <- errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	return signalC
}
