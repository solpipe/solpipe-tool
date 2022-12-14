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
	slt "github.com/solpipe/solpipe-tool/state/slot"
)

const MAX_TRIES_PERIOD_APPEND = 5
const DELAY_PERIOD_APPEND = 10 * time.Second
const PERIOD_START_TOO_CLOSE uint64 = 50

func (in *internal) run_period_append(event sch.Event) error {
	log.Debugf("received event=%s", event.String())
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

	// decrement because later we increment

	sh := in.router.Controller.SlotHome()
	var start uint64
	if in.slot+PERIOD_START_TOO_CLOSE < payload.Start {
		start = payload.Start
	} else {
		start = 0
	}
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

			return RunAppendPeriod(
				script,
				admin,
				sh,
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
		in.wrapper.ErrorNonNil(in.errorC),
	)
	return nil
}

func RunAppendPeriod(
	script *spt.Script,
	admin sgo.PrivateKey,
	sh slt.SlotHome,
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
	/*
		slot, err2 := sh.Time()
		if err2 != nil {
			return err2
		}
		// it takes 5 slots to get to the validator
		// typically the slot is off by 1
		if start < slot {
			start = slot + 50
		}
	*/
	err = script.SetTx(admin)
	if err != nil {
		return err
	}
	log.Debugf("run append period (%s;%d;%d;%d;%d;%d)", pipeline.Id.String(), 0, start, length, withhold, bidSpace)
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
	// this is time sensitive, so we skip simulation
	err = script.FinishTx(true)
	if err != nil {
		log.Debugf("tx submit failure: %s", err.Error())
		return err
	}
	return nil
}
