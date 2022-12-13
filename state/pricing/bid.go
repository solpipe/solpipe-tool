package pricing

import (
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
)

type periodInfo struct {
	period cba.Period
	bs     pyt.BidStatus
}

func (in *internal) on_bid(update bidUpdate) {
	node, present := in.periodM[update.payoutId.String()]
	if !present {
		return
	}
	node.Value().bs = update.bs
}
