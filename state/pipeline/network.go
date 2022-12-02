package pipeline

import ntk "github.com/solpipe/solpipe-tool/state/network"

func (in *internal) on_network_stats(ns ntk.NetworkStatus) {

}

func (in *internal) calculate_pipeline_tps() float64 {
	return in.stake_share() * in.networkStatus.AverageTransactionsPerSecond
}
