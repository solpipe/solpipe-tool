package router

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
)

func (in *internal) on_bid(list cba.BidList) {
	ref, present := in.l_payout.byId[list.Payout.String()]
	if !present {
		log.Debugf("pipeline not present (%s)", list.Payout.String())
		in.l_payout.bidListWithNoPayout[list.Payout.String()] = list
	} else {
		ref.p.UpdateBidList(list)
	}
}
