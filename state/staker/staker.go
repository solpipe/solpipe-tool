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
	ctx                 context.Context
	cancel              context.CancelFunc
	Stake               sgo.PublicKey
	internalC           chan<- func(*internal)
	dataStakerManagerC  chan<- sub.StakeGroup
	dataStakerReceiptC  chan<- sub.StakerReceiptGroup
	Id                  sgo.PublicKey // use stake member account id
	updateStakeManagerC chan<- dssub.ResponseChannel[cba.StakerManager]
	updateStakeReceiptC chan<- dssub.ResponseChannel[sub.StakerReceiptGroup]
}

func CreateStake(ctx context.Context, d sub.StakeGroup) (e1 Staker, err error) {
	internalC := make(chan func(*internal), 10)
	dataStakerManagerC := make(chan sub.StakeGroup, 10)
	dataStakerReceiptC := make(chan sub.StakerReceiptGroup, 10)
	//if d == nil {
	//	err = errors.New("no data")
	//	return
	//}
	stakerManagerHome := dssub.CreateSubHome[cba.StakerManager]()
	stakerReceiptHome := dssub.CreateSubHome[sub.StakerReceiptGroup]()
	updateStakeManagerC := stakerManagerHome.ReqC
	updateStakeReceiptC := stakerReceiptHome.ReqC
	ctxWithC, cancel := context.WithCancel(ctx)
	go loopInternal(
		ctxWithC,
		cancel,
		internalC,
		d,
		dataStakerManagerC,
		dataStakerReceiptC,
		stakerManagerHome,
		stakerReceiptHome,
	)

	e1 = Staker{
		ctx:                 ctxWithC,
		cancel:              cancel,
		internalC:           internalC,
		Stake:               d.Data.Stake,
		dataStakerManagerC:  dataStakerManagerC,
		dataStakerReceiptC:  dataStakerReceiptC,
		Id:                  d.Id,
		updateStakeManagerC: updateStakeManagerC,
		updateStakeReceiptC: updateStakeReceiptC,
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

type StakerData struct {
	Manager  cba.StakerManager
	Receipts []cba.StakerReceipt
}

func (sd StakerData) Stake() sgo.PublicKey {
	return sd.Manager.Stake
}

func (e1 Staker) Data() (ans StakerData, err error) {
	doneC := e1.ctx.Done()
	err = e1.ctx.Err()
	if err != nil {
		return
	}

	errorC := make(chan error, 1)
	ansC := make(chan StakerData, 1)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		if in.dataManager == nil {
			errorC <- errors.New("no receipt")
			return
		}
		list2 := make([]cba.StakerReceipt, len(in.dataReceiptById))
		i := 0
		for _, v := range in.dataReceiptById {
			list2[i] = v.Data
			i++
		}
		errorC <- nil
		ansC <- StakerData{
			Manager:  in.dataManager.Data,
			Receipts: list2,
		}
	}:
	}
	if err != nil {
		return
	}
	select {
	case err = <-errorC:
	case <-doneC:
		err = errors.New("canceled")
	}
	if err != nil {
		return
	}
	ans = <-ansC
	return

}

func (e1 Staker) OnData() dssub.Subscription[cba.StakerManager] {
	return dssub.SubscriptionRequest(e1.updateStakeManagerC, func(data cba.StakerManager) bool {
		return true
	})
}

func (e1 Staker) OnReceipt(receipt sgo.PublicKey) dssub.Subscription[sub.StakerReceiptGroup] {
	return dssub.SubscriptionRequest(e1.updateStakeReceiptC, func(obj sub.StakerReceiptGroup) bool {
		return receipt.Equals(obj.Data.Receipt)
	})
}
