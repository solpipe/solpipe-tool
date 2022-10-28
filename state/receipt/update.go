package receipt

import (
	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (e1 Receipt) Update(data cba.Receipt) {
	select {
	case <-e1.ctx.Done():
	case e1.dataC <- data:
	}
}

func (in *internal) on_data(data cba.Receipt) {
	in.receiptHome.Broadcast(data)
	in.data = &data
}

// Staker will delete itself if the stakemember account is closed.
func (e1 Receipt) UpdateStaker(obj sub.StakerReceiptGroup, d cba.StakerManager) {
	// stakers will not change from receipt
	select {
	case <-e1.ctx.Done():
	case e1.internalC <- func(in *internal) {
		in.on_staker(obj, d)
	}:
	}

}

type stakerInfo struct {
	id sgo.PublicKey
	r  cba.StakerReceipt
	d  cba.StakerManager
}

func (in *internal) on_staker(obj sub.StakerReceiptGroup, d cba.StakerManager) {
	managerId := obj.Data.Manager
	_, present := in.stakers[managerId.String()]
	if present {
		return
	}

	in.stakers[managerId.String()] = stakerInfo{
		id: obj.Id,
		d:  d,
		r:  obj.Data,
	}
	in.registeredStake += obj.Data.DelegatedStake

}
