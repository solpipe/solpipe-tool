package pricing

import (
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
)

type periodInfo struct {
	period       cba.Period
	bs           pyt.BidStatus
	pipelineInfo *pipelineInfo
}
