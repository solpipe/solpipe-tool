package cranker

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type PureCranker struct {
	Wrapper spt.Wrapper
}

// Wait until the period starts, then crank the BidList account to get the final bid deposit ratios
// Run one per payout
func CrankPayout(
	ctx context.Context,
	admin sgo.PrivateKey,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	wrapper spt.Wrapper,
	payout pyt.Payout,
	errorC chan<- error,
) {
	var err error

	log.Debugf("cranking(1) payout=%s", payout.Id.String())
	slot := uint64(0)

	doneC := ctx.Done()

	slotSub := controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()
	log.Debugf("cranking(2) payout=%s", payout.Id.String())
	bidSub := payout.OnBidStatus()
	defer bidSub.Unsubscribe()
	log.Debugf("cranking(3) payout=%s", payout.Id.String())
	data, err := payout.Data()
	if err != nil {
		sendError(errorC, err)
		log.Debugf("cranking(4) payout=%s", payout.Id.String())
		return
	}
	bidStatus, err := payout.BidStatus()
	if err != nil {
		sendError(errorC, err)
		log.Debugf("cranking(5) payout=%s", payout.Id.String())
		return
	}

	finish := data.Period.Start + data.Period.Length - 1
	log.Debugf("cranking(6) payout=%s", payout.Id.String())
out:
	for !(bidStatus.IsFinal || finish < slot) {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
		case err = <-bidSub.ErrorC:
			break out
		case bidStatus = <-bidSub.StreamC:
		}
	}
	log.Debugf("cranking(7) payout=%s", payout.Id.String())
	if err != nil {
		sendError(errorC, err)
		return
	}
	if bidStatus.IsFinal {
		log.Debugf("someone else already cranked payout=%s", payout.Id.String())
		return
	}
	log.Debugf("cranking(8) payout=%s", payout.Id.String())

	log.Debugf("now attempting to crank payout=%s with bidstatus=(%s,%d) and slots (finish=%d; slot=%d) ", payout.Id.String(), bidStatus.IsFinal, bidStatus.TotalDeposits, finish, slot)
	ctxC, cancel := context.WithCancel(ctx)
	go loopStopCrankOnBidChange(ctxC, cancel, bidStatus, payout)
	err = wrapper.Send(ctxC, 10, 15*time.Second, func(script *spt.Script) error {
		return runCrank(script, admin, controller, pipeline, payout)
	})
	cancel()
	log.Debugf("cranking(9) payout=%s", payout.Id.String())
	if err != nil {
		log.Debugf("cranking(10) payout=%s", payout.Id.String())
		log.Debug(err)
		return
	}
	log.Debugf("successfully cranked payout=%s", payout.Id.String())
}

// stop the crank if someone else manages to crank the bid list
func loopStopCrankOnBidChange(
	ctx context.Context,
	cancel context.CancelFunc,
	bidStatus pyt.BidStatus,
	payout pyt.Payout,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	bidSub := payout.OnBidStatus()
	defer bidSub.Unsubscribe()
out:
	for !bidStatus.IsFinal {
		select {
		case <-doneC:
			break out
		case err = <-bidSub.ErrorC:
			break out
		case bidStatus = <-bidSub.StreamC:
		}
	}
	if err != nil {
		log.Debug(err)
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
	err = script.FinishTx(true)
	if err != nil {
		return err
	}
	return nil
}

func sendError(errorC chan<- error, err error) {
	select {
	case <-time.After(30 * time.Second):
	case errorC <- err:
	}
}
