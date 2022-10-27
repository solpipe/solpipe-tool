package staker

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx        context.Context
	errorC     chan<- error
	data       *sub.StakeGroup
	stakerHome *dssub.SubHome[cba.StakerManager]
	cancel     context.CancelFunc
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	data sub.StakeGroup,
	dataC <-chan sub.StakeGroup,
	stakerHome *dssub.SubHome[cba.StakerManager],
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	in := new(internal)
	in.ctx = ctx
	in.cancel = cancel
	in.errorC = errorC
	in.data = &data
	in.stakerHome = stakerHome

	defer stakerHome.Close()

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
			if d.IsOpen {
				stakerHome.Broadcast(d.Data)
				in.data = &d
			} else {
				log.Debugf("exiting staker; stake member account closed")
				break out
			}
		case id := <-stakerHome.DeleteC:
			stakerHome.Delete(id)
		case d := <-stakerHome.ReqC:
			stakerHome.Receive(d)
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	in.cancel()
	log.Debug(err)
	// closing subscriptions indicate to subscribers that this staker account has been closed
	in.stakerHome.Close()
}
