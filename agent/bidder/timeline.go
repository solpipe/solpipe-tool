package bidder

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
)

type timeHome struct {
	line *ll.Generic[*timePoint]
	pob  map[string]*singlePayout // pipeline.id->singlePayout
}

type timePoint struct {
	Slot       uint64
	PayoutList map[string]*singlePayout // pipeline.id -> payout
}

type singlePayout struct {
	Period   *cba.PeriodWithPayout
	Pipeline *pipelineInfo
}

func (in *internal) init_timeline() {
	in.timeHome = new(timeHome)
	th := in.timeHome
	th.line = ll.CreateGeneric[*timePoint]()
	// in.pipelines
}

// Insert a period into the time line.
func (in *internal) timeline_on_period(pi *pipelineInfo, pr cba.PeriodRing) {
	th := in.timeHome
	tl := th.line
	pi.on_period(pr)
	size := uint16(len(pr.Ring))
	// periods are guaranteed to be in chronological order
	node := tl.HeadNode()
	var prevSlot uint64
	var curSlot uint64
	var gotNode bool
	var r *cba.PeriodWithPayout
	var k uint16
	for i := uint16(0); i < pr.Length; i++ {
		k = (i + pr.Start) % size
		r = &pr.Ring[k]
		gotNode = false
		// find out where on the timeline this new Period belongs
	gotnode:
		for n := node; n != nil; n = n.Next() {
			if n.Prev() != nil {
				prevSlot = n.Prev().Value().Slot
			} else {
				prevSlot = 0
			}
			curSlot = n.Value().Slot
			if prevSlot < r.Period.Start && r.Period.Start+r.Period.Length <= curSlot {
				node = n
				gotNode = true
				break gotnode
			}
		}
		if gotNode && r.Period.Start == node.Value().Slot {
			// we have a new point on the timeline
			sp, present := node.Value().PayoutList[pi.p.Id.String()]
			if !present {
				sp = &singlePayout{
					Period:   r,
					Pipeline: pi,
				}
				node.Value().PayoutList[pi.p.Id.String()] = sp
			} else {
				log.Debugf("duplicate entry for payout=%s", r.Payout.String())
			}
			in.timeline_period_new(node, sp)
		} else if gotNode {
			in.timeline_append(node.Prev(), pi, r)
		} else {
			in.timeline_append(tl.TailNode(), pi, r)
		}
	}
}

// we have a new period
func (in *internal) timeline_period_new(
	node *ll.Node[*timePoint],
	sp *singlePayout,
) {

	in.on_brain_tick()
}

func (in *internal) timeline_append(
	prevNode *ll.Node[*timePoint],
	pi *pipelineInfo,
	r *cba.PeriodWithPayout,
) {
	th := in.timeHome
	tl := th.line
	p := &timePoint{
		Slot:       r.Period.Start,
		PayoutList: make(map[string]*singlePayout),
	}
	node := tl.Insert(p, tl.TailNode())
	sp := &singlePayout{
		Pipeline: pi,
		Period:   r,
	}
	p.PayoutList[pi.p.Id.String()] = sp
	if node == nil {
		panic("should not be here")
	}
	in.timeline_period_new(node, sp)
}

func (in *internal) timeline_on_bid(pi *pipelineInfo, bl cba.BidList) {
	pi.on_bid(bl)
	// TODO: find the payout this bid book belongs to

	ourpk := in.bidder.PublicKey()
	var ourBid *cba.Bid
	list := ll.CreateGeneric[*cba.Bid]()
	deposit := uint64(0)
	var b *cba.Bid
	for _, bid := range bl.Book {
		b = &bid
		if !b.IsBlank {
			// do stuff here
			list.Append(b)
			deposit += b.Deposit
			if bid.User.Equals(ourpk) {
				ourBid = b
			}
		}
	}
	pi.ourBid = ourBid
	pi.totalDeposits = deposit
}

func (in *internal) on_slot() {
	th := in.timeHome
	tl := th.line
	var node *ll.Node[*timePoint]

gotnode:
	for node = tl.HeadNode(); node != nil; node = node.Next() {
		if node.Value().Slot < in.slot {
			tl.Remove(node)
			in.timeline_remove(node)
		} else {
			break gotnode
		}
	}
	in.timeline_bid_set()
}

// set which periods are being bid on now
func (in *internal) timeline_bid_set() {

	in.on_brain_tick()
}

// node has been removed from the time line
func (in *internal) timeline_remove(node *ll.Node[*timePoint]) {

	in.on_brain_tick()
}
