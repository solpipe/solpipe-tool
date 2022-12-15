package pricing

import (
	"math/big"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type stakeUpdate struct {
	pipelineId sgo.PublicKey
	relative   sub.StakeUpdate
}

func (in *internal) on_stake(r stakeUpdate) {
	pi, present := in.pipelineM[r.pipelineId.String()]
	if !present {
		return
	}
	if in.ns.AverageTransactionsPerSecond == 0 {
		pi.stats.tps = 0
		return
	}
	pi.stats_migrate()

	networkTps := big.NewFloat(in.ns.AverageTransactionsPerSecond)
	pi.stats.tps, _ = r.relative.Tps(networkTps).Float64()
	// TODO: calculate estimated capacity for this pipeline
}
