package pipeline

import (
	"errors"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpipe "github.com/solpipe/solpipe-tool/scheduler/pipeline"
	spt "github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

const MAX_TRIES_PERIOD_APPEND = 1
const DELAY_PERIOD_APPEND = 30 * time.Second

func (in *internal) run_period_append(event sch.Event) error {
	if in.periodSettings == nil {
		return errors.New("no period settings")
	}
	if in.rateSettings == nil {
		return errors.New("no rate settings")
	}
	payload, err := schpipe.ReadTrigger(event)
	if err != nil {
		return err
	}
	start := payload.Start - 1
	admin := in.admin
	length := in.periodSettings.Length
	withhold := uint16(in.periodSettings.Withhold)
	bidspace := uint16(in.periodSettings.BidSpace)
	controller := in.router.Controller
	pipeline := in.pipeline
	crankFee := state.RateFromProto(in.rateSettings.CrankFee)
	payoutShare := state.RateFromProto(in.rateSettings.PayoutShare)
	tickSize := uint16(in.periodSettings.TickSize)
	in.wrapper.SendDetached(
		sch.MergeCtx(in.ctx, payload.Context),
		MAX_TRIES_PERIOD_APPEND,
		DELAY_PERIOD_APPEND,
		func(script *spt.Script) error {
			start++
			return RunAppendPeriod(
				script,
				admin,
				start,
				length,
				withhold,
				bidspace,
				controller,
				pipeline,
				crankFee,
				payoutShare,
				tickSize,
			)
		},
		in.errorC,
	)
	return nil
}

func RunAppendPeriod(
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
