package pricing

import (
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
)

type periodInfo struct {
	period        cba.Period
	bs            pyt.BidStatus
	pipelineInfo  *pipelineInfo
	pipelineStats *pipelineStats // this points to the last calculated point; use this to calculate a delta
}

func (in *internal) on_bid(update bidUpdate) {
	node, present := in.periodM[update.payoutId.String()]
	if !present {
		return
	}
	perInfo := node.Value()
	perInfo.bs = update.bs
	// TODO: calculate estimated capacity for this pipeline
}
