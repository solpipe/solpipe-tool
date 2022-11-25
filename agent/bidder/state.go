package bidder

import (
	"math/big"
	"time"

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

func (in *internal) on_connection_status(x pipelineConnectionStatus) {
	pi, present := in.pipelines[x.id.String()]
	if !present {
		return
	}
	pi.status = &x
	if pi.status.err != nil {
		log.Debug(pi.status.err)
		nextDuration := 5 * time.Second
		if pi.lastDuration != 0 {
			nextDuration = 30 * time.Second
			//nextDuration = pi.lastDuration * 2
		}
		pi.lastDuration = nextDuration
		go loopDelayPleaseConnect(in.ctx, pi.pleaseConnectC, nextDuration)
	}
}
