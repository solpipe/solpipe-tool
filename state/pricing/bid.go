package pricing

import (
	"math/big"

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

func (in *internal) on_stake(r stakeUpdate) {
	pi, present := in.pipelineM[r.pipelineId.String()]
	if !present {
		return
	}
	if in.ns.AverageTransactionsPerSecond == 0 {
		pi.tps = 0
		return
	}
	networkTps := big.NewFloat(in.ns.AverageTransactionsPerSecond)
	pi.tps, _ = r.relative.Tps(networkTps).Float64()
}
