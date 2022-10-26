package script

import (
	"errors"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

func (e1 *Script) AddPipeline(
	controller ctr.Controller,
	payer sgo.PrivateKey,
	adminKey sgo.PrivateKey,
	crankFee state.Rate,
	allotment uint16,
	decayRate state.Rate,
	validatorPayoutShare state.Rate,
) (pipelineId sgo.PrivateKey, err error) {

	pipelineId, err = sgo.NewRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	return e1.AddPipelineDirect(
		pipelineId,
		controller,
		payer,
		adminKey,
		crankFee,
		allotment,
		decayRate,
		validatorPayoutShare,
	)
}

/* Add a pipeline auctioning TPS for a single send_tx proxy.  The payer here will pay rent on the period and bid accounts.
 */
func (e1 *Script) AddPipelineDirect(
	pipelineKeypair sgo.PrivateKey,
	controller ctr.Controller,
	payer sgo.PrivateKey,
	adminKey sgo.PrivateKey,
	crankFee state.Rate,
	allotment uint16,
	decayRate state.Rate,
	validatorPayoutShare state.Rate,
) (pipelineId sgo.PrivateKey, err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}

	controllerData, err := controller.Data()
	if err != nil {
		return
	}

	pipelineId = pipelineKeypair

	vaultId, _, err := vrs.PipelineVaultId(controller.Version, pipelineId.PublicKey())
	if err != nil {
		return nil, err
	}

	bidListId, err := e1.CreateAccount(util.STRUCT_SIZE_BID_LIST, cba.ProgramID, adminKey)
	if err != nil {
		return
	}
	periodRingId, err := e1.CreateAccount(util.STRUCT_SIZE_PERIOD_RING, cba.ProgramID, adminKey)
	if err != nil {
		return
	}

	log.Infof("add-pipeline----pipeline id=%s", pipelineId.PublicKey().String())

	b := cba.NewAddPipelineInstructionBuilder()
	b.SetControllerAccount(controller.Id())
	b.SetPcMintAccount(controllerData.PcMint)
	b.SetPipelineAccount(pipelineId.PublicKey())
	e1.AppendKey(pipelineId)
	b.SetPipelineVaultAccount(vaultId)
	b.SetBidsAccount(bidListId)
	b.SetPeriodsAccount(periodRingId)
	b.SetAdminAccount(adminKey.PublicKey())
	e1.AppendKey(adminKey)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetRentAccount(sgo.SysVarRentPubkey)

	b.SetAllotment(allotment)
	b.SetDecayRateNum(decayRate.N)
	b.SetDecayRateDen(decayRate.D)
	b.SetCrankFeeRateNum(crankFee.N)
	b.SetCrankFeeRateDen(crankFee.D)
	b.SetValidatorPayoutShareNum(validatorPayoutShare.N)
	b.SetValidatorPayoutShareDen(validatorPayoutShare.D)

	e1.txBuilder.AddInstruction(b.Build())

	return
}

func (e1 *Script) UpdatePipeline(
	controller sgo.PublicKey,
	pipelineId sgo.PublicKey,
	adminKey sgo.PrivateKey,
	crankFee state.Rate,
	allotment uint16,
	decayRate state.Rate,
	validatorPayoutShare state.Rate,
) (err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}

	b := cba.NewUpdatePipelineInstructionBuilder()

	b.SetControllerAccount(controller)
	b.SetPipelineAccount(pipelineId)
	b.SetAdminAccount(adminKey.PublicKey())
	e1.AppendKey(adminKey)
	b.SetNewAdminAccount(adminKey.PublicKey())
	e1.AppendKey(adminKey)

	b.SetAllotment(allotment)
	b.SetDecayRateNum(decayRate.N)
	b.SetDecayRateDen(decayRate.D)
	b.SetCrankFeeRateNum(crankFee.N)
	b.SetCrankFeeRateDen(crankFee.D)
	b.SetValidatorPayoutShareNum(validatorPayoutShare.N)
	b.SetValidatorPayoutShareDen(validatorPayoutShare.D)

	e1.txBuilder.AddInstruction(b.Build())

	return
}
