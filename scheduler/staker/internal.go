package staker

import (
	"context"

	log "github.com/sirupsen/logrus"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	skr "github.com/solpipe/solpipe-tool/state/staker"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	eventC           chan<- sch.Event
	closeSignalCList []chan<- error
	trackHome        *dssub.SubHome[sch.Event]
	pwd              pipe.PayoutWithData
	ps               sch.Schedule
	vs               sch.Schedule
	s                skr.Staker
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	vs sch.Schedule,
	s skr.Staker,
	trackHome *dssub.SubHome[sch.Event],
) {
	defer cancel()

	doneC := ctx.Done()
	errorC := make(chan error, 5)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.trackHome = trackHome

	var err error

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
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
