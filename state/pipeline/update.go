package pipeline

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type validatorStakeSub struct {
	sub    sub2.Subscription[val.StakeStatus]
	status *val.StakeStatus
	start  uint64
	finish uint64
}

type validatorStakeUpdate struct {
	vote   sgo.PublicKey
	status val.StakeStatus
	start  uint64
	finish uint64
}

func (e1 Pipeline) UpdateValidatorByVote(v val.Validator, start uint64, finish uint64) {
	go e1.loopUpdateValidatorByVote(v, start, finish)
	x := ValidatorUpdate{
		Validator: v, Start: start, Finish: finish,
	}
	select {
	case <-e1.ctx.Done():
	case e1.internalC <- func(in *internal) {
		in.validatorHome.Broadcast(x)
	}:
	}
}

func (e1 Pipeline) loopUpdateValidatorByVote(v val.Validator, start uint64, finish uint64) {

	d, err := v.Data()
	if err != nil {
		return
	}
	slotSub := e1.slotSub.OnSlot()
	defer slotSub.Unsubscribe()
	vote := d.Vote
	sub := v.OnStake()
	defer sub.Unsubscribe()
	presentC := make(chan bool, 1)
	errorC2 := make(chan chan<- error, 1)
	e1.internalC <- func(in *internal) {
		in.on_validator(d, presentC)
		errorC2 <- in.errorC
	}
	if <-presentC {

		<-errorC2
		return
	}
	errorC := <-errorC2
	doneC := e1.ctx.Done()
	slot := uint64(0)
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			if slot != 0 && finish <= slot {
				e1.updateValidatorStakeC <- validatorStakeUpdate{
					vote: vote,
					status: val.StakeStatus{
						Activated: 0,
						Total:     0,
					},
					start:  start,
					finish: finish,
				}
			}
		case err = <-sub.ErrorC:
			break out
		case s := <-sub.StreamC:
			e1.updateValidatorStakeC <- validatorStakeUpdate{
				vote:   vote,
				status: s,
				start:  start,
				finish: finish,
			}
		}
	}
	if err != nil {
		errorC <- err
	}
}

func (in *internal) on_validator(d cba.ValidatorManager, presentC chan<- bool) {
	x, present := in.validatorStakeSub[d.Vote.String()]
	presentC <- present
	if present {
		return
	}
	x = &validatorStakeSub{
		status: nil,
	}
	in.validatorStakeSub[d.Vote.String()] = x
}

// If you get IsOpen=true, then router will call e1.Close() for us. Do not close Pipeline here.
func (e1 Pipeline) Update(data cba.Pipeline) {
	e1.internalC <- func(in *internal) {
		in.on_data(data)
	}
}

// get alerted when the pipeline data has been changed
// If you get IsOpen=true, then router will call e1.Close() for us. Do not close Payout here.
func (in *internal) on_data(data cba.Pipeline) {
	in.data = &data
	in.pipelineHome.Broadcast(data)
}

func (e1 Pipeline) UpdatePeriod(ring cba.PeriodRing) {
	e1.internalC <- func(in *internal) {
		in.on_period(ring)
	}
}

func (e1 Pipeline) UpdatePayout(p pyt.Payout) {
	doneC := e1.ctx.Done()
	d, err := p.Data()
	if err != nil {
		log.Error(err)
		return
	}
	select {
	case <-doneC:
	case e1.internalC <- func(in *internal) {
		in.on_payout(p, d)
	}:
	}
}

// insert a new payout object
func (in *internal) on_payout(p pyt.Payout, d cba.Payout) {
	// linked list
	node := in.on_payout_insert(PayoutWithData{Payout: p, Data: d})
	if node == nil {
		return
	}
	in.payoutById[p.Id.String()] = node

	in.payoutHome.Broadcast(node.Value())

	go loopPipelinePayout(in.ctx, in.errorC, in.updatePayoutC, p)
	go loopDeletePayout(in.ctx, in.finishPayoutC, in.errorC, node.Value().Payout)
	in.on_payout_find_current()
}

func (in *internal) on_payout_insert(pwd PayoutWithData) *ll.Node[PayoutWithData] {
	list := in.payoutLinkedList

	if list.Size == 0 {
		return list.Append(pwd)
	}
	periodStart := pwd.Data.Period.Start
	periodEndPlusOne := periodStart + pwd.Data.Period.Length

	ans := new(ll.Node[PayoutWithData])
out:
	for node := list.HeadNode(); node != nil; node = node.Next() {
		if node.Value().Payout.Id.Equals(pwd.Payout.Id) {
			break out
		}
		nextNode := node.Next()
		if nextNode == nil {
			// reached end of list
			ans = list.Append(pwd)
			break out
		}

		if node.Value().Data.Period.Start+node.Value().Data.Period.Length <= periodStart && periodEndPlusOne <= nextNode.Value().Data.Period.Start {
			// insert in between
			ans = list.Insert(pwd, node)
			break out
		}

	}
	if ans == nil {
		log.Error("failed to insert payout")
	}
	return ans
}

// find out which payout is currently active (index=ring.Start)
func (in *internal) on_payout_find_current() {
	if in.periods == nil {
		return
	}
	if in.periods.Length == 0 {
		return
	}
	ring := in.periods.Ring
	start := in.periods.Start
	i := uint16(0)
	r := ring[(i+start)%uint16(len(ring))]

	node, present := in.payoutById[r.Payout.String()]
	if present {
		in.activePayout = node
	} else {
		in.activePayout = nil
	}

}

// get payout updates and delete payout from in.payoutLinkedList upon pyt.Payout exiting
func loopPipelinePayout(ctx context.Context, errorC chan<- error, updateC chan<- cba.Payout, p pyt.Payout) {
	sub := p.OnData()
	defer sub.Unsubscribe()
	doneC := ctx.Done()
	var err error
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case x := <-sub.StreamC:
			updateC <- x
		}
	}
	if err != nil {
		errorC <- err
	}
}

func (in *internal) on_payout_update(d cba.Payout) {
	node, present := in.payoutById[d.Pipeline.String()]
	if present {
		node.ChangeValue(PayoutWithData{
			Payout: node.Value().Payout,
			Data:   d,
		})
		in.payoutHome.Broadcast(node.Value())
	}
}

// wait until the payout object is deleted on chain, then delete the payout from the pipeline.
func loopDeletePayout(ctx context.Context, finishC chan<- payoutFinish, errorC chan<- error, payout pyt.Payout) {
	log.Debugf("setting up delete on payout=%s", payout.Id.String())
	id := payout.Id
	closeC := payout.CloseSignal()
	var err error
	doneC := ctx.Done()
	select {
	case <-doneC:
	case err = <-closeC:
		log.Debugf("payout=%s being deleted", id.String())
		if err != nil {
			errorC <- err
		}
		select {
		case <-doneC:
		case finishC <- payoutFinish{err: err, id: id}:
		}
	}

}

// get the currently active payout
func (e1 Pipeline) OnPayout() sub2.Subscription[PayoutWithData] {
	return sub2.SubscriptionRequest(e1.updatePayoutC, func(p PayoutWithData) bool { return true })
}

// get alerted when a new period has been appended
// The payout and period change together.
func (e1 Pipeline) OnPeriod() sub2.Subscription[cba.PeriodRing] {

	return sub2.SubscriptionRequest(e1.updatePeriodRingC, func(p cba.PeriodRing) bool { return true })
}

func (in *internal) on_period(ring cba.PeriodRing) {
	in.periods = &ring
	in.periodHome.Broadcast(ring)
	in.on_payout_find_current()
}

func (e1 Pipeline) UpdateBid(list cba.BidList) {
	e1.internalC <- func(in *internal) {
		in.on_bid(list)
	}
}

func (in *internal) on_bid(list cba.BidList) {
	in.bids = &list
	in.bidHome.Broadcast(list)
}
