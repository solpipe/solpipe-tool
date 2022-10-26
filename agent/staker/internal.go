package staker

import (
	"context"

	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	controller       ctr.Controller
	router           rtr.Router
	config           *Configuration
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	cancel context.CancelFunc,
	router rtr.Router,
	config *Configuration,
	sub *sgows.AccountSubscription,
) {
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.controller = router.Controller
	in.router = router

	//go loopStakeAccount(in.ctx, in.errorC, sub)
	streamC := sub.RecvStream()
	finishC := sub.RecvErr()

out:
	for {
		select {
		case err = <-finishC:
			break out
		case <-streamC:

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
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *internal) on_data(s cba.StakerMember) {

}
