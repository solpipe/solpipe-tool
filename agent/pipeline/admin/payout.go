package admin

import (
	"context"
	"os"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ckr "github.com/solpipe/solpipe-tool/agent/cranker"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type payoutInfo struct {
	m        map[uint64]*ll.Node[*payoutSingle]
	list     *ll.Generic[*payoutSingle]
	tailSlot uint64 // what is the last slot of the last period
}

type payoutSingle struct {
	pwd    pipe.PayoutWithData
	cancel context.CancelFunc
}

func (pi *payoutInfo) update_tail_slot() {
	tail, present := pi.list.Tail()
	if present {
		pi.tailSlot = tail.pwd.Data.Period.Start + tail.pwd.Data.Period.Length
	} else {
		pi.tailSlot = 0
	}
	log.Debugf("updating tail slot=%d", pi.tailSlot)
}

func (pi *payoutInfo) delete(start uint64) {
	node, present := pi.m[start]
	if present {
		return
	}
	node.Value().cancel()
	pi.list.Remove(node)
	delete(pi.m, start)
}

func (pi *payoutInfo) insert(ps *payoutSingle) *ll.Node[*payoutSingle] {
	_, present := pi.m[ps.pwd.Data.Period.Start]
	if present {
		return nil
	}
	start := ps.pwd.Data.Period.Start
	var node *ll.Node[*payoutSingle]
	head := pi.list.HeadNode()
	tail := pi.list.TailNode()
	if head == nil {
		log.Debugf("insert - 1")
		node = pi.list.Append(ps)
	} else if start < head.Value().pwd.Data.Period.Start {
		log.Debugf("insert - 2")
		node = pi.list.Prepend(ps)
	} else if tail.Value().pwd.Data.Period.Start < start {
		// this is covered in the next condition.  Keep this block anyway as a shortcut
		log.Debugf("insert - 3")
		node = pi.list.Append(ps)
	} else {
		log.Debugf("insert - 4")
		for n := pi.list.HeadNode(); n != nil; n = n.Next() {
			if n.Value().pwd.Data.Period.Start < start {
				node = pi.list.Insert(ps, n)
			}
		}
	}
	if node == nil {
		panic("ps should have been inserted by now")
	}
	pi.m[ps.pwd.Data.Period.Start] = node
	pi.update_tail_slot()
	return node
}

func (in *internal) init_payout() error {
	pi := new(payoutInfo)
	in.payoutInfo = pi
	pi.m = make(map[uint64]*ll.Node[*payoutSingle])
	pi.list = ll.CreateGeneric[*payoutSingle]()
	pi.tailSlot = 0
	return nil
}

func (in *internal) on_payout(pwd pipe.PayoutWithData) {
	if !pwd.Data.Pipeline.Equals(in.pipeline.Id) {
		return
	}
	pi := in.payoutInfo

	ps := &payoutSingle{pwd: pwd}
	node := pi.insert(ps)
	if node == nil {
		log.Debugf("have duplicate payout with start=%d", pwd.Data.Period.Start)
		return
	}

	ctxC, cancel := context.WithCancel(in.ctx)
	ps.cancel = cancel

	script, err := spt.Create(
		ctxC,
		&spt.Configuration{Version: in.controller.Version},
		in.rpc,
		in.ws,
	)
	if err != nil {
		in.errorC <- err
		return
	}
	wrapper := spt.Wrap(script)
	{
		bs, err := pwd.Payout.BidStatus()
		if err != nil {
			in.errorC <- err
			return
		}
		if !bs.IsFinal {
			log.Debugf("bid for payout=%s is NOT final", pwd.Id.String())
			go ckr.CrankPayout(
				in.ctx,
				in.admin,
				in.controller,
				in.pipeline,
				wrapper,
				pwd.Payout,
				in.errorC,
			)
		} else {
			log.Debugf("bid for payout=%s is final, skipping Crank", pwd.Id.String())
		}
	}

	go loopPayout(
		ctxC,
		cancel,
		in.errorC,
		in.deletePayoutC,
		in.controller,
		in.pipeline,
		pwd,
		wrapper,
		in.admin,
	)

}

type payoutInternal struct {
	ctx            context.Context
	slot           uint64
	errorC         chan<- error
	internalErrorC chan<- error
	deleteC        chan<- uint64
	controller     ctr.Controller
	pipeline       pipe.Pipeline
	data           *cba.Payout
	payout         pyt.Payout
	wrapper        spt.Wrapper
	admin          sgo.PrivateKey
	bidStatus      *pyt.BidStatus
}

func loopPayout(
	ctx context.Context,
	cancel context.CancelFunc,
	errorC chan<- error,
	deletePayoutC chan<- uint64,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	pwd pipe.PayoutWithData,
	wrapper spt.Wrapper,
	admin sgo.PrivateKey,
) {
	var err error
	defer cancel()
	slotHome := controller.SlotHome()
	doneC := ctx.Done()
	internalErrorC := make(chan error, 1)
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()
	bidSub := pwd.Payout.OnBidStatus()
	defer bidSub.Unsubscribe()

	pi := new(payoutInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.internalErrorC = internalErrorC
	pi.deleteC = deletePayoutC
	pi.controller = controller
	pi.pipeline = pipeline
	pi.data = &pwd.Data
	pi.payout = pwd.Payout
	pi.wrapper = wrapper
	pi.admin = admin
	pi.slot = 0

	pi.bidStatus = new(pyt.BidStatus)
	*pi.bidStatus, err = pi.payout.BidStatus()
	if err != nil {
		pi.errorC <- err
		return
	}

	finish := pi.data.Period.Start + pi.data.Period.Length + pyt.PAYOUT_CLOSE_DELAY + 1

	log.Debugf("finish for payout=%s is slot=(%d), validator count=%d ,with bid status=%+v", pi.payout.Id.String(), finish, pi.data.ValidatorCount, pi.bidStatus)

	attemptedCloseBid := false
	var closeBidCancel context.CancelFunc
out:
	for !(pi.bidStatus.IsFinal && finish <= pi.slot && pi.data.StakerCount == 0 && pi.data.ValidatorCount == 0) {
		select {
		case <-doneC:
			break out
		case err = <-internalErrorC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case *pi.data = <-dataSub.StreamC:
		case err = <-bidSub.ErrorC:
			break out
		case x := <-bidSub.StreamC:
			pi.on_bid(x)
		case err = <-slotSub.ErrorC:
			break out
		case pi.slot = <-slotSub.StreamC:
			if !attemptedCloseBid && pi.data.Period.Start <= pi.slot && pi.bidStatus.IsFinal {
				attemptedCloseBid = true
				// ignore errors from here
				// TODO: why does this instruction run even if the BidAccount has been closed?
				signalC := make(chan error, 1)
				closeBidCancel = pi.wrapper.SendDetached(pi.ctx, CLOSE_PAYOUT_MAX_TRIES, 30*time.Second, func(script *spt.Script) error {
					return runCloseBids(script, admin, controller, pipeline, pwd.Payout)
				}, signalC)
			}
			if pi.slot%50 == 0 {
				bm := "true"
				if !pi.bidStatus.IsFinal {
					bm = "false"
				}
				log.Debugf("pi (payout=%s)(final=%s)(slot=%d)(valcount=%d)(staker=%d)(finish=%d)", pi.payout.Id.String(), bm, pi.slot, pi.data.ValidatorCount, pi.data.StakerCount, finish)
			}
		}
	}
	if closeBidCancel != nil {
		closeBidCancel()
	}
	if err != nil {
		pi.finish(err)
		return
	}
	log.Debugf("payout id=%s has finished at slot=%d", pi.payout.Id.String(), pi.slot)
	payout := pi.payout
	err = pi.wrapper.Send(pi.ctx, CLOSE_PAYOUT_MAX_TRIES, 30*time.Second, func(s *spt.Script) error {
		return runClosePayout(s, admin, controller, pipeline, payout)
	})
	pi.finish(err)
}

func runCloseBids(
	script *spt.Script,
	admin sgo.PrivateKey,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	payout pyt.Payout,
) error {
	err := script.SetTx(admin)
	if err != nil {
		return err
	}
	err = script.CloseBids(
		controller,
		pipeline,
		payout,
		admin,
	)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	err = script.FinishTx(true)
	if err != nil {
		log.Debug("failed to close payout id=%s", payout.Id.String())
		os.Stderr.WriteString(err.Error() + "\n")
		return err
	}
	log.Debugf("payout id=%s has successfully been closed", payout.Id.String())
	return nil
}

func runClosePayout(
	script *spt.Script,
	admin sgo.PrivateKey,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	payout pyt.Payout,
) error {
	err := script.SetTx(admin)
	if err != nil {
		return err
	}
	err = script.CloseBids(
		controller,
		pipeline,
		payout,
		admin,
	)
	if err != nil {
		return err
	}
	err = script.ClosePayout(
		controller,
		pipeline,
		payout,
		admin,
	)
	if err != nil {
		return err
	}
	err = script.FinishTx(true)
	if err != nil {
		log.Debug("failed to close payout id=%s", payout.Id.String())
		os.Stderr.WriteString(err.Error() + "\n")
		return err
	}
	log.Debugf("payout id=%s has successfully been closed", payout.Id.String())
	return nil
}

func (pi *payoutInternal) on_bid(newStatus pyt.BidStatus) {
	*pi.bidStatus = newStatus
	if pi.bidStatus.IsFinal {
		log.Debugf("bid status for payout=%s is final", pi.payout.Id.String())
	}
}

func (pi *payoutInternal) finish(err error) {
	doneC := pi.ctx.Done()
	err2 := pi.ctx.Err()
	if err2 != nil {
		return
	}
	if err != nil {
		select {
		case <-doneC:
			return
		case pi.errorC <- err:
		}
	}

	select {
	case <-doneC:
	case pi.deleteC <- pi.data.Period.Start:
	}
}

const CLOSE_PAYOUT_MAX_TRIES = 10
