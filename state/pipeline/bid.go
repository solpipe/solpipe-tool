package pipeline

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	"github.com/solpipe/solpipe-tool/util"
)

// get alerted when a bid has been inserted
func (e1 Pipeline) OnBid() dssub.Subscription[cba.BidList] {
	return dssub.SubscriptionRequest(e1.updateBidListC, func(b cba.BidList) bool { return true })
}

type bidInfo struct {
	bidders         map[string]*ll.Node[*BidStatus] // map user_id ->bid
	allocationTotal uint64
	lastPeriodStart uint64
	list            ll.Generic[*BidStatus]
}

func (in *internal) init_bid_status() {
	in.bidInfo = new(bidInfo)
	in.bidInfo.bidders = make(map[string]*ll.Node[*BidStatus])
	in.bidInfo.allocationTotal = 0
	in.bidInfo.lastPeriodStart = 0
}

func (in *internal) on_bid_bucket_update(oldbids *cba.BidList, newbids *cba.BidList) {
	if oldbids != nil {
		if oldbids.LastPeriodStart == newbids.LastPeriodStart || !newbids.AllocationIsFinal {
			return
		}
	}
	bi := in.bidInfo

	allocationTotal := uint64(0)
	for _, bid := range newbids.Book {
		allocationTotal += bid.BandwidthAllocation
	}
	if allocationTotal == 0 {
		// every bidder's allocation is being zero-ed out
		bi.allocationTotal = 1
	}

	var present bool
	var allocation float64
	var bid *cba.Bid
	var node *ll.Node[*BidStatus]
	pipelineTps := in.calculate_pipeline_tps()
	for i := 0; i < len(newbids.Book); i++ {
		bid = &newbids.Book[i]
		if !bid.IsBlank {
			node, present = bi.bidders[bid.User.String()]
			if !present {
				allocation = float64(bid.BandwidthAllocation) / float64(bi.allocationTotal)
				bs := &BidStatus{
					Bid:             *bid,
					Allocation:      allocation,
					AllotedTps:      allocation * pipelineTps,
					LastPeriodStart: newbids.LastPeriodStart,
				}
				var err error
				node = bi.list.InsertSorted(bs, func(before, after *ll.Node[*BidStatus], newItem *BidStatus) bool {
					var c int
					var d int
					if before == nil && after != nil {
						c = ll.ComparePublicKey(newItem.Bid.User, after.Value().Bid.User)
						if c < 0 {
							return true
						} else if c == 0 {
							err = errors.New("duplicate")
							return false
						} else {
							return false
						}
					} else if before != nil && after == nil {
						c = ll.ComparePublicKey(newItem.Bid.User, before.Value().Bid.User)
						if c < 0 {
							return false
						} else if c == 0 {
							err = errors.New("duplicate")
							return false
						} else {
							return true
						}
					} else {
						c = ll.ComparePublicKey(newItem.Bid.User, before.Value().Bid.User)
						d = ll.ComparePublicKey(newItem.Bid.User, after.Value().Bid.User)
						if 0 < c && d < 0 {
							return true
						} else if c == 0 || d == 0 {
							err = errors.New("duplicate")
							return false
						} else {
							return false
						}
					}
				})
				if err != nil {
					in.errorC <- err
					return
				}
				if node == nil {
					in.errorC <- errors.New("failed to insert node")
					return
				}
				bi.bidders[bid.User.String()] = node
				in.bidStatusHome.Broadcast(*bs)
			} else {
				bs := node.Value()
				oldAllotedTps := bs.AllotedTps
				bs.Bid = *bid
				bs.Allocation = float64(bid.BandwidthAllocation) / float64(bi.allocationTotal)
				bs.AllotedTps = bs.Allocation * pipelineTps
				if oldAllotedTps != bs.AllotedTps {
					in.bidStatusHome.Broadcast(*bs)
				}

			}

		}
	}
	// zero out all bidders that are not in the latest bid list
	bi.list.Iterate(func(x *BidStatus, index uint32, deleteNode func()) error {
		if x.LastPeriodStart != newbids.LastPeriodStart {
			x.AllotedTps = 0
			x.Allocation = 0
			in.bidStatusHome.Broadcast(*x)
		}
		return nil
	})

}

func (in *internal) on_network_stats(ns ntk.NetworkStatus) {
	*in.networkStatus = ns
	if in.bidInfo == nil {
		return
	}
	if in.bids == nil {
		return
	}
	pipelineTps := in.calculate_pipeline_tps()
	in.bidInfo.list.Iterate(func(x *BidStatus, index uint32, delete func()) error {
		x.AllotedTps = x.Allocation * pipelineTps
		if x.Allocation != 0 {
			in.bidStatusHome.Broadcast(*x)
		}
		return nil
	})
}

func (in *internal) calculate_pipeline_tps() float64 {
	return in.stake_share() * in.networkStatus.AverageTransactionsPerSecond
}

type BidStatus struct {
	Bid             cba.Bid
	Allocation      float64
	AllotedTps      float64
	LastPeriodStart uint64
}

// Get the TPS alloted to each bidder
func (e1 Pipeline) OnBidStatus(user sgo.PublicKey) dssub.Subscription[BidStatus] {
	return dssub.SubscriptionRequest(e1.updateBidStatusC, func(bs BidStatus) bool {
		if bs.Bid.User.Equals(user) {
			return true
		} else if user.Equals(util.Zero()) {
			return true
		}
		return false
	})
}
