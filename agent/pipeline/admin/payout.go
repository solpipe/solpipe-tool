package admin

import (
	"context"
	"errors"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ckr "github.com/solpipe/solpipe-tool/agent/cranker"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
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
	ctx                       context.Context
	slot                      uint64
	errorC                    chan<- error
	internalErrorC            chan<- error
	deleteC                   chan<- uint64
	controller                ctr.Controller
	pipeline                  pipe.Pipeline
	data                      *cba.Payout
	payout                    pyt.Payout
	wrapper                   spt.Wrapper
	admin                     sgo.PrivateKey
	hasStarted                bool
	hasFinished               bool
	isReadyToClose            bool
	bidIsFinal                bool
	bidHasClosed              bool
	validatorAddingHasStarted bool
	validatorAddingIsDone     bool
	cancelCrank               context.CancelFunc
	cancelCloseBid            context.CancelFunc
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
	//bidSub := pwd.Payout.OnBidStatus()
	//defer bidSub.Unsubscribe()

	eventC := make(chan PayoutEvent)
	clockPeriodStartC := make(chan bool, 1)
	go loopClock(ctx, controller, eventC, errorC, clockPeriodStartC, pwd.Data)
	go loopPayoutEvent(ctx, pwd, errorC, eventC, clockPeriodStartC)

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
	pi.hasStarted = false
	pi.hasFinished = false
	pi.isReadyToClose = false
	pi.bidIsFinal = false
	pi.bidHasClosed = false
	pi.validatorAddingHasStarted = false
	pi.validatorAddingIsDone = false

	// pi.errorC will return a nil error in pi.errorC from pi.run_close_payout()
	// thereby exiting this forloop

	var isTrans string
out:
	for {
		select {
		case <-doneC:
			break out
		case event := <-eventC:
			if event.IsStateChange {
				isTrans = "change"
			} else {
				isTrans = "static"
			}
			log.Debugf("event payout=%s  type=%s  isTransition=%s", pi.payout.Id.String(), event.Type, isTrans)
			switch event.Type {
			case EVENT_START:
				err = pi.on_start(event.IsStateChange)
			case EVENT_FINISH:
				err = pi.on_finish(event.IsStateChange)
			case EVENT_CLOSE_OUT:
				err = pi.on_close_out(event.IsStateChange)
			case EVENT_BID_CLOSED:
				err = pi.on_bid_closed(event.IsStateChange)
			case EVENT_BID_FINAL:
				err = pi.on_bid_final(event.IsStateChange)
			case EVENT_VALIDATOR_IS_ADDING:
				err = pi.on_validator_is_adding(event.IsStateChange)
			case EVENT_VALIDATOR_HAVE_WITHDRAWN:
				err = pi.on_validator_have_withdrawn(event.IsStateChange)
			default:
				err = errors.New("unknown event")
				break out
			}
		}

	}
	pi.finish(err)
}

// clockPeriodStartC bool indicates if this is a state transition
func loopPayoutEvent(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	errorC chan<- error,
	eventC chan<- PayoutEvent,
	clockPeriodStartC <-chan bool,
) {
	var err error
	doneC := ctx.Done()

	dataSub := pwd.Payout.OnData()
	defer dataSub.Unsubscribe()

	newData := pwd.Data

	zero := util.Zero()
	ctxBid, cancelBid := context.WithCancel(ctx)
	defer cancelBid()
	bidsAreClosed := false
	if newData.Bids.Equals(zero) {
		bidsAreClosed = true
		select {
		case <-doneC:
			return
		case eventC <- pevent(EVENT_BID_CLOSED, false, 0):
		}
		cancelBid()
	} else {
		go loopBidSubIsFinal(ctxBid, pwd, errorC, eventC)
	}

	validatorHasAdded := false
	if 0 < pwd.Data.ValidatorCount {
		validatorHasAdded = true
	}
	validatorHasWithdrawn := false

out:
	for !bidsAreClosed && !validatorHasAdded && !validatorHasWithdrawn {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case isStateTransition := <-clockPeriodStartC:
			// period has started
			if !validatorHasAdded {
				validatorHasAdded = true
				select {
				case <-doneC:
					return
				case eventC <- pevent(EVENT_VALIDATOR_IS_ADDING, isStateTransition, 0):
				}
				if newData.ValidatorCount == 0 {
					validatorHasWithdrawn = true
					select {
					case <-doneC:
						return
					case eventC <- pevent(EVENT_VALIDATOR_HAVE_WITHDRAWN, isStateTransition, 0):
					}
				}
			}
		case newData = <-dataSub.StreamC:
			// check if BidClosed
			if !bidsAreClosed && newData.Bids.Equals(zero) {
				bidsAreClosed = true
				select {
				case <-doneC:
					return
				case eventC <- pevent(EVENT_BID_CLOSED, true, 0):
				}
				cancelBid()
			}

			// check if validators are signing up or cashing out
			if !validatorHasAdded && 0 < newData.ValidatorCount {
				validatorHasAdded = true
				select {
				case <-doneC:
					return
				case eventC <- pevent(EVENT_VALIDATOR_IS_ADDING, true, 0):
				}
			}
			if validatorHasAdded && !validatorHasWithdrawn && newData.ValidatorCount == 0 {
				validatorHasWithdrawn = true
				select {
				case <-doneC:
					return
				case eventC <- pevent(EVENT_VALIDATOR_HAVE_WITHDRAWN, true, 0):
				}
			}
		}
	}

	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}

func loopBidSubIsFinal(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	errorC chan<- error,
	eventC chan<- PayoutEvent,
) {
	var err error
	doneC := ctx.Done()
	bidSub := pwd.Payout.OnBidStatus()
	defer bidSub.Unsubscribe()

	bsIsFinal := false
	bs, err := pwd.Payout.BidStatus()
	if err != nil {
		errorC <- err
		return
	} else if bs.IsFinal {
		bsIsFinal = true
		select {
		case <-doneC:
			return
		case eventC <- PayoutEvent{
			Slot: 0, Type: EVENT_BID_FINAL,
		}:
		}
	}
out:
	for bsIsFinal {
		select {
		case <-doneC:
			break out
		case err = <-bidSub.ErrorC:
			break out
		case bs = <-bidSub.StreamC:
			if !bsIsFinal && bs.IsFinal {
				bsIsFinal = true
				select {
				case <-doneC:
					break out
				case eventC <- PayoutEvent{
					Slot: 0,
					Type: EVENT_BID_FINAL,
				}:
				}

			}
		}
	}
	if err != nil {
		errorC <- err
		return
	}
}

func loopClock(
	ctx context.Context,
	controller ctr.Controller,
	eventC chan<- PayoutEvent,
	errorC chan<- error,
	clockPeriodStartC chan<- bool,
	payoutData cba.Payout,
) {
	var err error
	var slot uint64
	doneC := ctx.Done()
	slotSub := controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()
	start := payoutData.Period.Start
	sentStart := false
	isStartStateTransition := false
	finish := start + payoutData.Period.Length - 1
	sentFinish := false
	isFinishStateTransition := false
	closeOut := finish + PAYOUT_POST_FINISH_DELAY
	sentClose := false
	isCloseStateTransition := false
out:
	for !sentStart && !sentFinish && !sentClose {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			if !sentStart && start <= slot {
				sentStart = true
				select {
				case <-doneC:
					break out
				case eventC <- pevent(EVENT_START, isStartStateTransition, slot):
				}
			} else if !isStartStateTransition {
				isStartStateTransition = true
			}
			if !sentFinish && finish <= slot {
				sentFinish = true
				select {
				case <-doneC:
					break out
				case eventC <- pevent(EVENT_FINISH, isFinishStateTransition, slot):
				}
			} else if !isFinishStateTransition {
				isFinishStateTransition = true
			}
			if !sentClose && closeOut <= slot {
				sentClose = true
				select {
				case <-doneC:
					break out
				case eventC <- pevent(EVENT_CLOSE_OUT, isCloseStateTransition, slot):
				}
			} else if !isCloseStateTransition {
				isCloseStateTransition = true
			}
		}
	}
	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}

func runCrank(
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
	err = script.Crank(
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
		log.Debugf("failed to crank payout=%s", payout.Id.String())
		os.Stderr.WriteString(err.Error() + "\n")
		return err
	}
	log.Debugf("payout id=%s has successfully been cranked (bid is final)", payout.Id.String())
	return nil
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
		log.Debugf("failed to close bids payout=%s", payout.Id.String())
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
const CLOSE_BIDS_MAX_TRIES = 10
const CRANK_MAX_TRIES = 10
