package admin

import (
	"context"
	"errors"
	"math"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	spt "github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

const RETRY_INTERVAL = uint64(1500)
const START_SLOT_BUFFER = uint64(150)

func (in *internal) attempt_add_period() {

	tail := in.payoutInfo.tailSlot // start+length
	lookaheadLimit := in.slot + in.periodSettings.Lookahead
	start := tail
	if start < in.slot {
		// add buffer to give time for us to get the tx to the validator before the slot passes the start
		start = in.slot + START_SLOT_BUFFER
	}
	if !(!in.appendInProgress && start < lookaheadLimit) {
		return
	}
	log.Debugf("slot=%d; tail=%d; start=%d; lookahead=%d;", in.slot, tail, start, lookaheadLimit)
	in.appendInProgress = true
	in.lastAddPeriodAttemptToAddPeriod = in.slot
	log.Debugf("(slot=%d) attempting to add a period; start=%d; end=%d; bid space=%d", in.slot, start, start+in.periodSettings.Length-1, in.periodSettings.BidSpace)
	if math.MaxUint16 <= in.periodSettings.Withhold {
		in.errorC <- errors.New("withhold is too large")
		return
	}
	if math.MaxUint16 <= in.periodSettings.BidSpace {
		in.errorC <- errors.New("bid space is too large")
		return
	}
	if in.periodSettings.BidSpace <= 10 {
		in.errorC <- errors.New("bid space is too small")
		return
	}

	script, err := spt.Create(in.ctx, &spt.Configuration{Version: in.controller.Version}, in.rpc, in.ws)
	if err != nil {
		in.errorC <- err
		return
	}
	admin := in.admin
	length := in.periodSettings.Length
	withhold := uint16(in.periodSettings.Withhold)
	bidSpace := uint16(in.periodSettings.BidSpace)
	controller := in.controller
	pipeline := in.pipeline
	crankFee := state.RateFromProto(in.rateSettings.CrankFee)
	payoutShare := state.RateFromProto(in.rateSettings.PayoutShare)
	tickSize := uint16(in.periodSettings.TickSize)
	wrapper := spt.Wrap(script)
	signalC := make(chan error, 1)
	_, cancel := wrapper.SendDetached(in.ctx, 5, 10*time.Second, func(s *spt.Script) error {
		return runAppendPeriod(
			s,
			admin,
			start,
			length,
			withhold,
			bidSpace,
			controller,
			pipeline,
			crankFee,
			payoutShare,
			tickSize,
		)
	}, signalC)
	go loopAppendErrorToError(in.ctx, cancel, in.pipeline, in.appendInProgressC, signalC, in.errorC)

}

func loopAppendErrorToError(
	ctx context.Context,
	cancel context.CancelFunc,
	pipeline pipe.Pipeline,
	progressC chan<- bool,
	resultC <-chan error,
	errorC chan<- error,
) {
	doneC := ctx.Done()
	// subscribe to OnPeriod change to give time for the state router to get all the relevant updates from the
	periodSub := pipeline.OnPeriod()
	defer periodSub.Unsubscribe()
	var err error

	select {
	case <-doneC:
	case err = <-periodSub.ErrorC:
	case err = <-resultC:
	}

	if err == nil {
		select {
		case err = <-periodSub.ErrorC:
		case <-doneC:
		case <-periodSub.StreamC:
		}
	}

	if err != nil {
		log.Debugf("failed to append: %s", err.Error())
		select {
		case <-doneC:
		case errorC <- err:
		}
	} else {
		log.Debug("append successful")
		select {
		case <-doneC:
		case progressC <- false:
		}
	}
}

func runAppendPeriod(
	script *spt.Script,
	admin sgo.PrivateKey,
	start uint64,
	length uint64,
	withhold uint16,
	bidSpace uint16,
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	crankFee state.Rate,
	payoutShare state.Rate,
	tickSize uint16,
) error {
	var err error
	err = script.SetTx(admin)
	if err != nil {
		return err
	}

	err = script.UpdatePipeline(
		controller.Id(),
		pipeline.Id,
		admin,
		crankFee,
		pipe.ALLOTMENT_DEFAULT,
		payoutShare,
		tickSize,
	)
	if err != nil {
		log.Debugf("update pipeline failure: %s", err.Error())
		return err
	}

	_, err = script.AppendPeriod(
		controller,
		pipeline,
		admin,
		start,
		length,
		withhold,
		bidSpace,
	)
	if err != nil {
		log.Debugf("append error: %s", err.Error())
		return err
	}
	// a successful transaction here does not imply that the transaction has made it into a block and run without error
	err = script.FinishTx(true)
	if err != nil {
		log.Debugf("tx submit failure: %s", err.Error())
		return err
	}

	return nil
}
