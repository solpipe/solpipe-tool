package admin

import (
	"context"
	"os"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	"github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type payoutInfo struct {
	m    map[uint64]*ll.Node[*payoutSingle]
	list *ll.Generic[*payoutSingle]
}

type payoutSingle struct {
	pwd    pipe.PayoutWithData
	cancel context.CancelFunc
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
		node = pi.list.Append(ps)
	} else if start < head.Value().pwd.Data.Period.Start {
		node = pi.list.Prepend(ps)
	} else if tail.Value().pwd.Data.Period.Start < start {
		// this is covered in the next condition.  Keep this block anyway as a shortcut
		node = pi.list.Append(ps)
	} else {
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
	return node
}

func (in *internal) init_payout() error {
	pi := new(payoutInfo)
	in.payoutInfo = pi
	pi.m = make(map[uint64]*ll.Node[*payoutSingle])
	pi.list = ll.CreateGeneric[*payoutSingle]()
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

	script, err := script.Create(
		ctxC,
		&script.Configuration{Version: in.controller.Version},
		in.rpc,
		in.ws,
	)
	if err != nil {
		in.errorC <- err
	}

	go loopPayout(
		ctxC,
		cancel,
		in.errorC,
		in.deletePayoutC,
		in.controller,
		in.pipeline,
		pwd,
		script,
		in.admin,
	)

	go loopDeletePayout(in.ctx, in.errorC, in.deletePayoutC, pwd.Payout)

}

type payoutInternal struct {
	ctx        context.Context
	slot       uint64
	errorC     chan<- error
	deleteC    chan<- sgo.PublicKey
	controller ctr.Controller
	pipeline   pipe.Pipeline
	data       *cba.Payout
	payout     pyt.Payout
	script     *script.Script
	admin      sgo.PrivateKey
	bidStatus  *pyt.BidStatus
}

func loopPayout(
	ctx context.Context,
	cancel context.CancelFunc,
	errorC chan<- error,
	deletePayoutC chan<- sgo.PublicKey,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	pwd pipe.PayoutWithData,
	script *script.Script,
	admin sgo.PrivateKey,
) {
	var err error
	defer cancel()
	slotHome := controller.SlotHome()
	doneC := ctx.Done()
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()
	bidSub := pwd.Payout.OnBidStatus()
	defer bidSub.Unsubscribe()

	pi := new(payoutInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.deleteC = deletePayoutC
	pi.controller = controller
	pi.pipeline = pipeline
	pi.data = &pwd.Data
	pi.payout = pwd.Payout
	pi.script = script
	pi.admin = admin

	pi.bidStatus = new(pyt.BidStatus)
	*pi.bidStatus, err = pi.payout.BidStatus()
	if err != nil {
		pi.errorC <- err
		return
	}

	finish := pi.data.Period.Start + pi.data.Period.Length + pyt.PAYOUT_CLOSE_DELAY

	log.Debugf("finish for payout=%s is slot=%d", pi.payout.Id.String(), finish)
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case *pi.data = <-dataSub.StreamC:
		case err = <-bidSub.ErrorC:
			break out
		case x := <-bidSub.StreamC:
			pi.on_bid(&x)
		case err = <-slotSub.ErrorC:
			break out
		case pi.slot = <-slotSub.StreamC:
			if pi.slot%100 == 0 {
				log.Debugf("pi.slot(%s)=%d", pi.payout.Id.String(), pi.slot)
			}
		}
	}
	log.Debugf("payout id=%s has finished at slot=%d", pi.payout.Id.String(), pi.slot)

	if err == nil {
		delay := 1 * time.Second
	close:
		for tries := 0; tries < CLOSE_PAYOUT_MAX_TRIES; tries++ {
			time.Sleep(delay)
			delay = 30 * time.Second
			err = pi.close_payout()
			if err == nil {
				break close
			}
		}

	}

	pi.finish(err)
}

func (pi *payoutInternal) on_bid(newStatus *pyt.BidStatus) {
	oldStatus := pi.bidStatus
	if oldStatus.IsFinal {
		return
	}
	if newStatus.IsFinal {

	}

	if oldStatus.IsFinal {
		return
	}

	pi.bidStatus = newStatus
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
	case pi.deleteC <- pi.payout.Id:
	}
}

const CLOSE_PAYOUT_MAX_TRIES = 10

func (pi *payoutInternal) close_payout() error {
	log.Debugf("attempting to close payout=%s", pi.payout.Id.String())
	err := pi.script.SetTx(pi.admin)
	if err != nil {
		return err
	}
	err = pi.script.ClosePayout(
		pi.controller,
		pi.pipeline,
		pi.payout,
		pi.admin,
	)
	if err != nil {
		return err
	}
	err = pi.script.FinishTx(true)
	if err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		return err
	}
	log.Debugf("payout id=%s has successfully been closed", pi.payout.Id.String())
	return nil
}
