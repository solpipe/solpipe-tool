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

func (in *internal) attempt_add_period(
	start uint64,
) {

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
	cancel := wrapper.SendDetached(in.ctx, 1, 0*time.Second, func(s *spt.Script) error {
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
	go loopAppendErrorToError(in.ctx, cancel, in.appendInProgressC, signalC, in.errorC)

}

func loopAppendErrorToError(
	ctx context.Context,
	cancel context.CancelFunc,
	progressC chan<- bool,
	resultC <-chan error,
	errorC chan<- error,
) {
	doneC := ctx.Done()
	select {
	case <-doneC:
	case err := <-resultC:
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
	err = script.FinishTx(true)
	if err != nil {
		log.Debugf("tx submit failure: %s", err.Error())
		return err
	}
	return nil
}
