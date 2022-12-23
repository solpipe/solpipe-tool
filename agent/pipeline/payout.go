package pipeline

import (
	"context"
	"os"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	ckr "github.com/solpipe/solpipe-tool/agent/cranker"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpyt "github.com/solpipe/solpipe-tool/scheduler/payout"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type payoutInfo struct {
	cancel context.CancelFunc
	pwd    pipe.PayoutWithData
	s      sch.Schedule
}

func (in *internal) on_payout(pwd pipe.PayoutWithData) {
	_, present := in.payoutM[pwd.Id.String()]
	if !present {
		pi := new(payoutInfo)
		var ctxC context.Context
		ctxC, pi.cancel = context.WithCancel(in.ctx)
		pi.pwd = pwd
		pi.s = schpyt.Schedule(ctxC, in.router, pwd)
		in.payoutM[pwd.Id.String()] = pi
		go loopDeletePayout(in.ctx, ctxC, in.deletePayoutC, pwd.Id)
		go loopPayoutScheduler(ctxC, pi.s, in.errorC, in.eventC)
	}
}

func loopPayoutScheduler(
	ctx context.Context,
	s sch.Schedule,
	errorC chan<- error,
	eventC chan<- sch.Event,
) {
	var err error

	doneC := ctx.Done()
	sub := s.OnEvent()
	defer sub.Unsubscribe()
	var event sch.Event
	log.Debugf("loop payout scheduler")
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case event = <-sub.StreamC:
			log.Debugf("payout scheduler event=%s", event.String())
			select {
			case <-doneC:
				break out
			case eventC <- event:
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

func loopDeletePayout(
	parentCtx context.Context,
	payoutCtx context.Context,
	deleteC chan<- sgo.PublicKey,
	id sgo.PublicKey,
) {
	parentDoneC := parentCtx.Done()
	payoutDoneC := payoutCtx.Done()
	select {
	case <-parentDoneC:
		return
	case <-payoutDoneC:
	}
	select {
	case <-parentDoneC:
	case deleteC <- id:
	}
}

func (in *internal) run_payout_crank(event sch.Event) error {
	trigger, err := schpyt.ReadTrigger(event)
	if err != nil {
		return err
	}

	go ckr.CrankPayout(
		sch.MergeCtx(in.ctx, trigger.Context),
		in.admin,
		in.router.Controller,
		in.pipeline,
		in.wrapper,
		trigger.Payout,
		in.wrapper.ErrorNonNil(in.errorC),
	)
	return nil
}

const MAX_TRIES_PAYOUT_CLOSE_BIDS = 5
const DELAY_PAYOUT_CLOSE_BIDS = 30 * time.Second

func (in *internal) run_payout_close_bids(event sch.Event) error {
	trigger, err := schpyt.ReadTrigger(event)
	if err != nil {
		return err
	}
	admin := in.admin
	controller := in.router.Controller
	pipeline := in.pipeline
	ctxC := trigger.Context
	payout := trigger.Payout
	in.wrapper.SendDetached(
		sch.MergeCtx(in.ctx, ctxC),
		MAX_TRIES_PAYOUT_CLOSE_BIDS,
		DELAY_PAYOUT_CLOSE_BIDS,
		func(script *spt.Script) error {
			return RunCloseBids(
				script,
				admin,
				controller,
				pipeline,
				payout,
			)
		},
		in.wrapper.ErrorNonNil(in.errorC),
	)
	return nil
}

func RunCloseBids(
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

func (in *internal) run_payout_close_payout(event sch.Event) error {
	trigger, err := schpyt.ReadTrigger(event)
	if err != nil {
		return err
	}
	// no trigger context for payout
	admin := in.admin
	controller := in.router.Controller
	pipeline := in.pipeline
	payout := trigger.Payout
	in.wrapper.SendDetached(
		in.ctx,
		MAX_TRIES_PAYOUT_CLOSE_PAYOUT,
		DELAY_PAYOUT_CLOSE_PAYOUT,
		func(script *spt.Script) error {
			return RunClosePayout(
				script,
				admin,
				controller,
				pipeline,
				payout,
			)
		},
		in.wrapper.ErrorNonNil(in.errorC),
	)
	return nil
}

const MAX_TRIES_PAYOUT_CLOSE_PAYOUT = 5
const DELAY_PAYOUT_CLOSE_PAYOUT = 30 * time.Second

func RunClosePayout(
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
		log.Debugf("failed to close payout id=%s", payout.Id.String())
		os.Stderr.WriteString(err.Error() + "\n")
		return err
	}
	log.Debugf("payout id=%s has successfully been closed", payout.Id.String())
	return nil
}
