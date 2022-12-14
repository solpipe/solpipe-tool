package pipeline

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type validatorStakeSub struct {
	sub    dssub.Subscription[val.StakeStatus]
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

func (in *internal) on_validator(d cba.ValidatorManager, presentC chan<- bool) {
	_, present := in.validatorStakeSub[d.Vote.String()]
	presentC <- present
	if present {
		return
	}

	in.validatorStakeSub[d.Vote.String()] = &validatorStakeSub{
		status: nil,
	}
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
	log.Debugf("router add payout=%s", p.Id.String())
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
	node := in.on_payout_insert(PayoutWithData{Id: p.Id, Payout: p, Data: d})
	if node == nil {
		return
	}
	log.Debugf("list=%+v", in.payoutLinkedList)
	in.payoutById[p.Id.String()] = node

	in.payoutHome.Broadcast(node.Value())

	go loopPipelinePayout(in.ctx, in.errorC, in.updatePayoutC, p)
	go loopDeletePayout(in.ctx, in.finishPayoutC, in.errorC, node.Value().Payout)
	in.on_payout_find_current()
}

func (in *internal) on_payout_insert(pwd PayoutWithData) *ll.Node[PayoutWithData] {
	log.Debugf("inserting id=%s", pwd.Id.String())
	list := in.payoutLinkedList

	if list.Size == 0 {
		return list.Append(pwd)
	}
	periodStart := pwd.Data.Period.Start
	//periodEndPlusOne := periodStart + pwd.Data.Period.Length

	//var ans *ll.Node[PayoutWithData]

	tail := list.TailNode()
	if tail.Value().Data.Period.Start < pwd.Data.Period.Start {
		return list.Append(pwd)
	}

	for node := list.HeadNode(); node != nil; node = node.Next() {
		if node.Value().Payout.Id.Equals(pwd.Payout.Id) {
			node.ChangeValue(pwd)
			return node
		}
		if periodStart < node.Value().Data.Period.Start {
			list.Insert(node.Value(), node)
			// switch with old node
			node.ChangeValue(pwd)
			return node
		}
	}
	log.Error("failed to insert payout")
	return nil
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

// get the current admin
func (e1 Pipeline) OnData() dssub.Subscription[cba.Pipeline] {
	return dssub.SubscriptionRequest(e1.updatePipelineC, func(p cba.Pipeline) bool { return true })
}

// get the currently active payout
func (e1 Pipeline) OnPayout() dssub.Subscription[PayoutWithData] {
	return dssub.SubscriptionRequest(e1.updatePayoutC, func(p PayoutWithData) bool { return true })
}

// get alerted when a new period has been appended
// The payout and period change together.
func (e1 Pipeline) OnPeriod() dssub.Subscription[cba.PeriodRing] {
	return dssub.SubscriptionRequest(e1.updatePeriodRingC, func(p cba.PeriodRing) bool { return true })
}

func (in *internal) on_period(ring cba.PeriodRing) {
	in.periods = &ring
	in.periodHome.Broadcast(ring)
	in.on_payout_find_current()
}
