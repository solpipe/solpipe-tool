package payout

import (
	"errors"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
)

// get alerted when a bid has been inserted
func (e1 Payout) OnBidStatus() dssub.Subscription[BidStatus] {
	return dssub.SubscriptionRequest(e1.updateBidStatusC, func(b BidStatus) bool { return true })
}

type bidInfo struct {
	is_final      bool
	totalDeposits uint64
	list          *ll.Generic[*cba.Bid]
	search        map[string]*ll.Node[*cba.Bid] // user.id->bid
}

func (in *internal) init_bid() error {
	bi := new(bidInfo)
	bi.list = ll.CreateGeneric[*cba.Bid]()
	bi.search = make(map[string]*ll.Node[*cba.Bid])
	bi.is_final = false
	in.bi = bi
	return nil
}

// update the linked list so that it only contains up-to-date
func (in *internal) on_bid_list(bl cba.BidList) {
	bi := in.bi
	if bi.is_final {
		log.Error("we should not be here")
		return
	}
	bi.totalDeposits = bl.TotalDeposits
	bi.is_final = bl.BiddingFinished
	newList := ll.CreateGeneric[*cba.Bid]()

	oldList := bi.list
	var oldNode *ll.Node[*cba.Bid]
	var present bool
	for _, bid := range bl.Book {
		if !bid.IsBlank {
			oldNode, present = bi.search[bid.User.String()]
			if present {
				oldList.Remove(oldNode)
				bi.search[bid.User.String()] = newList.Append(oldNode.Value())
			} else {
				bi.search[bid.User.String()] = newList.Append(&bid)
			}
		}
	}
	if 0 < oldList.Size {
		oldList.Iterate(func(obj *cba.Bid, index uint32, deleteNode func()) error {
			if !obj.IsBlank {
				delete(bi.search, obj.User.String())
			}
			return nil
		})
	}
	bi.list = newList

	in.bid_broadcast()
}

func (in *internal) bid_broadcast() {
	bi := in.bi
	list := make([]cba.Bid, bi.list.Size)
	bi.list.Iterate(func(obj *cba.Bid, index uint32, deleteNode func()) error {
		list[index] = *obj
		return nil
	})
	in.bidStatusHome.Broadcast(BidStatus{
		Bid:           list,
		IsFinal:       bi.is_final,
		TotalDeposits: bi.totalDeposits,
	})
}

func (in *internal) bid_status() BidStatus {
	list := make([]cba.Bid, in.bi.list.Size)
	in.bi.list.Iterate(func(obj *cba.Bid, index uint32, delete func()) error {
		list[index] = *obj
		return nil
	})
	return BidStatus{
		Bid:           list,
		IsFinal:       in.bi.is_final,
		TotalDeposits: in.bi.totalDeposits,
	}
}

type BidStatus struct {
	Bid           []cba.Bid // only contains active bids
	IsFinal       bool
	TotalDeposits uint64
}

func (e1 Payout) BidStatus() (bs BidStatus, err error) {
	doneC := e1.ctx.Done()
	ansC := make(chan BidStatus, 1)
	errorC := make(chan error, 1)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		errorC <- nil
		ansC <- in.bid_status()
	}:
	}
	select {
	case err = <-errorC:
	case bs = <-ansC:
	}

	return
}
