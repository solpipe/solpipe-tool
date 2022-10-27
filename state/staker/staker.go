package staker

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type Staker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	internalC    chan<- func(*internal)
	dataC        chan<- sub.StakeGroup
	Id           sgo.PublicKey // use stake member account id
	updateStakeC chan<- dssub.ResponseChannel[cba.StakerManager]
}

func CreateStake(ctx context.Context, d sub.StakeGroup) (e1 Staker, err error) {
	internalC := make(chan func(*internal), 10)
	dataC := make(chan sub.StakeGroup, 10)
	//if d == nil {
	//	err = errors.New("no data")
	//	return
	//}
	stakerHome := dssub.CreateSubHome[cba.StakerManager]()
	updateStakeC := stakerHome.ReqC
	ctxWithC, cancel := context.WithCancel(ctx)
	go loopInternal(ctxWithC, cancel, internalC, d, dataC, stakerHome)

	e1 = Staker{
		ctx:          ctxWithC,
		cancel:       cancel,
		internalC:    internalC,
		dataC:        dataC,
		Id:           d.Id,
		updateStakeC: updateStakeC,
	}
	return
}

// Close when the on chain account has been closed
func (e1 Staker) Close() {
	e1.cancel()
}

func (e1 Staker) OnClose() <-chan struct{} {
	return e1.ctx.Done()
}

func (e1 Staker) Data() (ans cba.StakerManager, err error) {
	err = e1.ctx.Err()
	if err != nil {
		return
	}
	doneC := e1.ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan cba.StakerManager, 1)
	e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no receipt")
		} else {
			errorC <- nil
			ansC <- in.data.Data
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
	return

}

func (e1 Staker) OnData() dssub.Subscription[cba.StakerManager] {
	return dssub.SubscriptionRequest(e1.updateStakeC, func(data cba.StakerManager) bool {
		return true
	})
}
