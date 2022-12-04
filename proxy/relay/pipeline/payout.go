package pipeline

import (
	"context"

	log "github.com/sirupsen/logrus"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
)

type periodInfo struct {
	list *ll.Generic[*payoutWithBidStatus]
	m    map[uint64]*ll.Node[*payoutWithBidStatus]
}

func (pi *periodInfo) find(start uint64) (exact *ll.Node[*payoutWithBidStatus], appendTarget *ll.Node[*payoutWithBidStatus]) {
	tail := pi.list.TailNode()
	if tail.Value() == nil {
		return nil, nil
	} else if tail.Value().Pwd.Data.Period.Start < start {
		return nil, nil
	}

	for node := pi.list.HeadNode(); node != nil; node = node.Next() {
		// nStart is monotonically increasing
		nStart := node.Value().Pwd.Data.Period.Start
		if nStart == start {
			return node, nil
		} else if start < nStart {
			return nil, node
		}
	}
	return nil, nil // implies that we need to append
}

func (in *internal) init_period() error {
	pi := new(periodInfo)
	in.periodInfo = pi
	pi.list = ll.CreateGeneric[*payoutWithBidStatus]()
	pi.m = make(map[uint64]*ll.Node[*payoutWithBidStatus])
	return nil
}

type payoutWithBidStatus struct {
	Pwd    pipe.PayoutWithData
	Status pyt.BidStatus
	cancel context.CancelFunc
	bi     *bidderInfo
}

func (in *internal) pbs_create(pwd pipe.PayoutWithData) (*payoutWithBidStatus, error) {
	bs, err := pwd.Payout.BidStatus()
	if err != nil {
		return nil, err
	}
	ctxC, cancel := context.WithCancel(in.ctx)
	pwbs := &payoutWithBidStatus{
		Pwd:    pwd,
		Status: bs,
		cancel: cancel,
	}
	if !bs.IsFinal {
		go loopBidStatusUpdate(
			ctxC,
			in.errorC,
			in.bidStatusC,
			pwd,
		)
	}

	go loopDeletePayout(
		ctxC,
		in.errorC,
		in.deletePayoutC,
		in.slotHome,
		pwd.Data.Period.Start,
		pwd.Data.Period.Start+pwd.Data.Period.Length,
	)

	err = in.pwbs_init_bid(pwbs)
	if err != nil {
		cancel()
		return nil, err
	}

	return pwbs, nil
}

func (in *internal) on_payout(pwd pipe.PayoutWithData, slotHome slot.SlotHome) {
	if pwd.Data.Period.Start+pwd.Data.Period.Length <= in.slot {
		log.Debugf("payout with start=%d has already expired", pwd.Data.Period.Start)
		return
	}
	pi := in.periodInfo
	exact, appendTarget := pi.find(pwd.Data.Period.Start)
	if exact != nil {
		exact.Value().Pwd = pwd
	} else if appendTarget != nil {
		pbs, err := in.pbs_create(pwd)
		if err != nil {
			in.errorC <- err
			return
		}
		pi.list.Insert(pbs, appendTarget)
	} else {
		pbs, err := in.pbs_create(pwd)
		if err != nil {
			in.errorC <- err
			return
		}
		pi.list.Append(pbs)
	}

}

func (in *internal) pwbs_init_bid(pwbs *payoutWithBidStatus) error {
	bi := new(bidderInfo)
	bi.list = ll.CreateGeneric[*bidderFeed]()
	bi.m = make(map[string]*ll.Node[*bidderFeed])
	return nil
}

type bidStatusWithStartTime struct {
	start  uint64
	status pyt.BidStatus
}

func loopBidStatusUpdate(
	ctx context.Context,
	errorC chan<- error,
	outC chan<- bidStatusWithStartTime,
	pwd pipe.PayoutWithData,
) {
	doneC := ctx.Done()
	bidStatusSub := pwd.Payout.OnBidStatus()
	defer bidStatusSub.Unsubscribe()

	var err error

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-bidStatusSub.ErrorC:
			break out
		case x := <-bidStatusSub.StreamC:
			select {
			case <-doneC:
				break out
			case outC <- bidStatusWithStartTime{
				start:  pwd.Data.Period.Start,
				status: x,
			}:
			}
		}
	}
	if err != nil {
		errorC <- err
	}
}

func loopDeletePayout(
	ctx context.Context,
	errorC chan<- error,
	deleteC chan<- uint64,
	slotHome slot.SlotHome,
	start uint64,
	end uint64,
) {
	doneC := ctx.Done()
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	var err error
	var slot uint64
out:
	for {
		select {
		case <-doneC:
			return
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			if end <= slot {
				break out
			}
		}
	}
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
	} else {
		select {
		case <-doneC:
		case deleteC <- start:
		}
	}

}
