package validator

import (
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schval "github.com/solpipe/solpipe-tool/scheduler/validator"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	val "github.com/solpipe/solpipe-tool/state/validator"
	"github.com/solpipe/solpipe-tool/util"
)

func (in *internal) on_event(event sch.Event) error {
	var err error
	switch event.Type {
	case sch.TRIGGER_VALIDATOR_SET_PAYOUT:
		err = in.run_validator_set_payout(event)
	case sch.TRIGGER_VALIDATOR_WITHDRAW_RECEIPT:
		err = in.run_validator_withdraw_receipt(event)
	default:
		log.Debugf("received unmatched event: %s", event.String())
	}
	return err
}

const MAX_TRIES_VALIDATOR_SET_PAYOUT = 5

func (in *internal) run_validator_set_payout(event sch.Event) error {
	trigger, err := schval.ReadTrigger(event)
	if err != nil {
		return err
	}

	controllerId := in.controller.Id()
	payoutId := trigger.Payout.Id
	pipelineId := trigger.Pipeline.Id
	validatorId := in.validator.Id
	validatorAdmin := in.config.Admin

	log.Debugf(
		"validator set payout: (%s;%s;%s;%s;%s)",
		controllerId.String(),
		payoutId.String(),
		pipelineId.String(),
		validatorId.String(),
		validatorAdmin.PublicKey().String(),
	)

	in.scriptWrapper.SendDetached(
		util.MergeCtx(in.ctx, trigger.Context),
		MAX_TRIES_VALIDATOR_SET_PAYOUT,
		5*time.Second,
		func(script *spt.Script) error {
			return runValidatorSetPayout(
				script,
				controllerId,
				payoutId,
				pipelineId,
				validatorId,
				validatorAdmin,
			)
		},
		in.scriptWrapper.ErrorNonNil(in.errorC),
	)
	return nil
}

func runValidatorSetPayout(
	script *spt.Script,
	controllerId sgo.PublicKey,
	payoutId sgo.PublicKey,
	pipelineId sgo.PublicKey,
	validatorId sgo.PublicKey,
	validatorAdmin sgo.PrivateKey,
) error {
	err := script.SetTx(validatorAdmin)
	if err != nil {
		return err
	}
	_, err = script.ValidatorSetPipeline(
		controllerId,
		payoutId,
		pipelineId,
		validatorId,
		validatorAdmin,
	)
	if err != nil {
		return err
	}
	err = script.FinishTx(true)
	if err != nil {
		log.Debugf("run set validator failed: %s", err.Error())
		return err
	}
	log.Debug("successfully set validator to payout")
	return nil
}

const MAX_TRIES_VALIDATOR_WITHDRAW_RECEIPT = 10

func (in *internal) run_validator_withdraw_receipt(event sch.Event) error {
	trigger, err := schval.ReadTrigger(event)
	if err != nil {
		return err
	}

	controller := in.controller
	payoutId := trigger.Payout.Id
	pipeline := trigger.Pipeline
	validator := in.validator
	validatorAdmin := in.config.Admin
	receiptId := trigger.Receipt.Id
	in.scriptWrapper.SendDetached(
		util.MergeCtx(in.ctx, trigger.Context),
		MAX_TRIES_VALIDATOR_WITHDRAW_RECEIPT,
		30*time.Second,
		func(script *spt.Script) error {
			return runValidatorWithdrawReceipt(
				script,
				validatorAdmin,
				controller,
				payoutId,
				pipeline,
				validator,
				receiptId,
			)
		},
		in.scriptWrapper.ErrorNonNil(in.errorC),
	)

	return nil
}

func runValidatorWithdrawReceipt(
	script *spt.Script,
	validatorAdmin sgo.PrivateKey,
	controller ctr.Controller,
	payoutId sgo.PublicKey,
	pipeline pipe.Pipeline,
	validator val.Validator,
	receiptId sgo.PublicKey,
) error {
	err := script.SetTx(validatorAdmin)
	if err != nil {
		return err
	}
	err = script.ValidatorWithdrawReceipt(
		controller,
		payoutId,
		pipeline,
		validator,
		receiptId,
		validatorAdmin,
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
