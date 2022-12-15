package pricing

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
)

type periodInfo struct {
	period       cba.Period
	bs           pyt.BidStatus
	pipelineInfo *pipelineInfo
	projectedTps float64
	requiredTps  float64
}

func (in *internal) on_bid(update bidUpdate) {
	node, present := in.periodM[update.payoutId.String()]
	if !present {
		return
	}
	pr := node.Value()
	oldBs := pr.bs
	pr.bs = update.bs
	oldShare, err := oldBs.Share(in.bidder)
	if err != nil {
		in.errorC <- err
		return
	}
	newShare, err := pr.bs.Share(in.bidder)
	if err != nil {
		in.errorC <- err
		return
	}
	log.Debugf("share changed by %f for payout=%s", newShare-oldShare, pr.pipelineInfo.p.Id.String())
	err = in.update_tps(pr)
	if err != nil {
		in.errorC <- err
		return
	}
}
