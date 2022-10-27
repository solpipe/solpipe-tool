package staker

import (
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (e1 Staker) Update(d sub.StakeGroup) {

	select {
	case <-e1.ctx.Done():
	case e1.dataStakerManagerC <- d:
	}
}

func (e1 Staker) UpdateReceipt(d sub.StakerReceiptGroup) {
	select {
	case <-e1.ctx.Done():
	case e1.dataStakerReceiptC <- d:
	}

}
