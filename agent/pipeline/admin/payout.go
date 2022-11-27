package admin

import (
	"context"
	"os"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
)

func (in *internal) on_payout(pwd pipe.PayoutWithData) {

	slotHome := in.controller.SlotHome()

	script, err := script.Create(
		in.ctx,
		&script.Configuration{Version: in.controller.Version},
		in.rpc,
		in.ws,
	)
	if err != nil {
		in.errorC <- err
	}

	go loopPayout(
		in.ctx,
		in.errorC,
		in.deletePayoutC,
		slotHome,
		in.controller,
		in.pipeline,
		pwd,
		script,
		in.admin,
	)

}

type payoutInternal struct {
	ctx        context.Context
	slot       uint64
	errorC     chan<- error
	deleteC    chan<- sgo.PublicKey
	controller ctr.Controller
	pipeline   pipe.Pipeline
	data       cba.Payout
	payout     pyt.Payout
	script     *script.Script
	admin      sgo.PrivateKey
}

func loopPayout(
	ctx context.Context,
	errorC chan<- error,
	deletePayoutC chan<- sgo.PublicKey,
	slotHome slot.SlotHome,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	pwd pipe.PayoutWithData,
	script *script.Script,
	admin sgo.PrivateKey,
) {
	var err error

	doneC := ctx.Done()
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()

	pi := new(payoutInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.deleteC = deletePayoutC
	pi.controller = controller
	pi.pipeline = pipeline
	pi.data = pwd.Data
	pi.payout = pwd.Payout
	pi.script = script
	pi.admin = admin

	finish := pi.data.Period.Start + pi.data.Period.Length + pyt.PAYOUT_CLOSE_DELAY

	log.Debugf("finish for payout=%s is slot=%d", pi.payout.Id.String(), finish)
out:
	for pi.slot < finish {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case pi.slot = <-slotSub.StreamC:
			if pi.slot%100 == 0 {
				log.Debugf("pi.slot(%s)=%d", pi.payout.Id.String(), pi.slot)
			}
		}
	}
	if finish <= pi.slot {
		log.Debugf("payout id=%s has finished at slot=%d", pi.payout.Id.String(), pi.slot)
	}

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
	// finish here
	pi.finish(err)
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
