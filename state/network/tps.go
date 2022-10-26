package network

import (
	"context"

	ll "github.com/solpipe/solpipe-tool/ds/list"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	log "github.com/sirupsen/logrus"
)

type NetworkStatus struct {
	WindowSize                       uint32 // how many blocks is the average calculated
	AverageTransactionsPerBlock      float64
	AverageTransactionsPerSecond     float64
	AverageTransactionsSizePerSecond float64 // bytes per second
}

type tpsInternal struct {
	ctx              context.Context
	list             *ll.Generic[BlockTransactionCount]
	status           *NetworkStatus
	networkStatsHome *sub2.SubHome[NetworkStatus]
}

func loopTps(
	ctx context.Context,
	cancel context.CancelFunc,
	networkStatsHome *sub2.SubHome[NetworkStatus],
	blockReadSub sub2.Subscription[BlockTransactionCount],
) {
	defer cancel()

	in := new(tpsInternal)
	in.ctx = ctx
	in.list = ll.CreateGeneric[BlockTransactionCount]()
	in.networkStatsHome = networkStatsHome
	in.status = &NetworkStatus{
		WindowSize:                       20,
		AverageTransactionsPerBlock:      0,
		AverageTransactionsPerSecond:     0,
		AverageTransactionsSizePerSecond: 0,
	}

out:
	for {
		select {
		case <-ctx.Done():
			break out
		case id := <-networkStatsHome.DeleteC:
			networkStatsHome.Delete(id)
		case r := <-networkStatsHome.ReqC:
			networkStatsHome.Receive(r)
		case <-blockReadSub.ErrorC:
			break out
		case b := <-blockReadSub.StreamC:
			in.list.Append(b)
			in.update()
		}
	}

}

func (in *tpsInternal) update() {
	if in.list.Size < 2 {
		return
	}

	var dT float64
	var startSlot uint64
	var lastSlot uint64
	sumFees := uint64(0)
	sumSize := uint64(0)
	sumCount := uint64(0)
	in.list.Iterate(func(obj BlockTransactionCount, index uint32, delete func()) error {
		if index == 0 {
			startSlot = obj.Slot
			return nil
		}
		lastSlot = obj.Slot
		sumFees += obj.Fees
		sumSize += obj.TransactionSize
		sumCount += obj.TransactionCount
		return nil
	})
	dT = float64(lastSlot) - float64(startSlot)
	if dT < 0 {
		log.Debug("negative slot differential")
		return
	}
	in.status.AverageTransactionsPerBlock = float64(sumCount) / float64(in.list.Size-1)
	in.status.AverageTransactionsPerSecond = float64(sumCount) / dT
	in.status.AverageTransactionsSizePerSecond = float64(sumSize) / dT

	in.networkStatsHome.Broadcast(*in.status)
}
