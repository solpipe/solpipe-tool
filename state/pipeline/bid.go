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
	list            *ll.Generic[*BidStatus]
	//lookupIndex     *radix.Tree[*ll.Node[*BidStatus]]
}

func (in *internal) init_bid_status() {
	in.bidInfo = new(bidInfo)
	in.bidInfo.bidders = make(map[string]*ll.Node[*BidStatus])
	in.bidInfo.allocationTotal = 0
	in.bidInfo.lastPeriodStart = 0
	in.bidInfo.list = ll.CreateGeneric[*BidStatus]()
}

// TODO: find some better algorithm for inserting/updating bids
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
				node = bi.list.Append(bs)
				if node == nil {
					in.errorC <- errors.New("failed to insert node")
					return
				}
				bi.bidders[bid.User.String()] = node
				in.bidStatusHome.Broadcast(*bs)
			} else {
				bs := node.Value()
				oldAllotedTps := bs.AllotedTps
				bs.LastPeriodStart = newbids.LastPeriodStart
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
			x.LastPeriodStart = newbids.LastPeriodStart
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

func (e1 Pipeline) AllBidStatus(user sgo.PublicKey) (list []BidStatus, err error) {
	err = e1.ctx.Err()
	if err != nil {
		return
	}
	doneC := e1.ctx.Done()
	countC := make(chan uint32)
	ansC := make(chan BidStatus, 10)
	errorC := make(chan error, 1)

	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.all_bid_status(countC, ansC, errorC)
	}:
	}

	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	case count := <-countC:
		list = make([]BidStatus, count)
	}
	if err != nil {
		return
	}
out:
	for i := 0; i < len(list); i++ {
		select {
		case <-doneC:
			err = errors.New("canceled")
			break out
		case err = <-errorC:
			break out
		case list[i] = <-ansC:
		}
	}

	return
}

func (in *internal) all_bid_status(countC chan<- uint32, ansC chan<- BidStatus, errorC chan<- error) {
	doneC := in.ctx.Done()
	countC <- in.bidInfo.list.Size
	errorC <- in.bidInfo.list.Iterate(func(x *BidStatus, index uint32, deleteNode func()) error {
		var err2 error
		select {
		case <-doneC:
			err2 = errors.New("canceled")
		case ansC <- *x:
		}
		return err2
	})

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
