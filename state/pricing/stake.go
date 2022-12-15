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

	oldStats := pi.stats_migrate()
	if oldStats == nil {
		oldStats = &pipelineStats{
			tps: 0,
		}
	}
	networkTps := big.NewFloat(in.ns.AverageTransactionsPerSecond)
	pi.stats.tps, _ = r.relative.Tps(networkTps).Float64()
	err := pi.propagate_tps(pi.stats.tps - oldStats.tps)
	if err != nil {
		in.errorC <- err
		return
	}
}

func (pi *pipelineInfo) propagate_tps(deltaTps float64) error {
	var err error
	for node := pi.periodList.HeadNode(); node != nil; node = node.Next() {
		err = pi.in.update_tps(node.Value())
		if err != nil {
			return err
		}
	}
	// Calculate dif between required capacity and projectedTps
	return nil
}
