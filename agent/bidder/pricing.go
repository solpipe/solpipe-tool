package bidder

import (
	"math/big"

	log "github.com/sirupsen/logrus"
)

func (in *internal) on_relative_stake() {
	if in.networkStatus == nil {
		log.Debug("no network tps statistics")
		return
	}
	activeStake := big.NewFloat(0)
	activeStake.SetInt(in.activeStake)
	totalStake := big.NewFloat(0)
	totalStake.SetInt(in.totalStake)
	tps := big.NewFloat(0)
	tps.SetFloat64(in.networkStatus.AverageTransactionsPerSecond)
	ratio := big.NewFloat(0)
	in.allotedTps.Mul(
		tps, ratio.Quo(activeStake, totalStake),
	)
	f, _ := in.allotedTps.Float64()
	log.Debugf("new relative stake for pipeline: %f", f)
}
