package payout

import (
	"context"

	log "github.com/sirupsen/logrus"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
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
	eventHome *dssub.SubHome[sch.Event],
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 5)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
