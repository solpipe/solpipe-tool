package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/util"
)

func (e1 *Script) AddPipeline(
	controller ctr.Controller,
	payer sgo.PrivateKey,
	adminKey sgo.PrivateKey,
	crankFee state.Rate,
	allotment uint16,
	validatorPayoutShare state.Rate,
	tickSize uint16,
	refundSpace uint16,
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
		validatorPayoutShare,
		tickSize,
		refundSpace,
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
	validatorPayoutShare state.Rate,
	tickSize uint16,
	refundSpace uint16,
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

	refundsId, err := e1.CreateAccount(e1.refundSize(refundSpace), cba.ProgramID, adminKey)
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
	b.SetRefundsAccount(refundsId)
	b.SetPeriodsAccount(periodRingId)
	b.SetAdminAccount(adminKey.PublicKey())
	e1.AppendKey(adminKey)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetRentAccount(sgo.SysVarRentPubkey)

	log.Debugf("allotment=%d", allotment)
	b.SetAllotment(allotment)
	log.Debugf("crank fee=%d/%d", crankFee.N, crankFee.D)
	b.SetCrankFeeRateNum(crankFee.N)
	b.SetCrankFeeRateDen(crankFee.D)
	log.Debugf("payout share=%d/%d", validatorPayoutShare.N, validatorPayoutShare.D)
	b.SetValidatorPayoutShareNum(validatorPayoutShare.N)
	b.SetValidatorPayoutShareDen(validatorPayoutShare.D)
	log.Debugf("tick size=%d", tickSize)
	b.SetTickSize(tickSize)
	log.Debugf("refund space=%d", refundSpace)
	b.SetRefundSpace(refundSpace)
	b.SetReceiptLimit(RECEIPT_LIMIT_DEFUALT)

	e1.txBuilder.AddInstruction(b.Build())

	return
}

func (e1 *Script) refundSize(refundSpace uint16) uint64 {
	return util.STRUCT_SIZE_REFUND_HEADER + uint64(refundSpace)*util.STRUCT_SIZE_REFUND_CLAIM
}

func (e1 *Script) UpdatePipeline(
	controller sgo.PublicKey,
	pipelineId sgo.PublicKey,
	adminKey sgo.PrivateKey,
	crankFee state.Rate,
	allotment uint16,
	validatorPayoutShare state.Rate,
	tickSize uint16,
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
	b.SetCrankFeeRateNum(crankFee.N)
	b.SetCrankFeeRateDen(crankFee.D)
	b.SetValidatorPayoutShareNum(validatorPayoutShare.N)
	b.SetValidatorPayoutShareDen(validatorPayoutShare.D)
	b.SetTickSize(tickSize)
	b.SetReceiptLimit(RECEIPT_LIMIT_DEFUALT) //TODO: make this variable

	e1.txBuilder.AddInstruction(b.Build())

	return
}

const TICKSIZE_DEFAULT uint16 = 1

const BIDSPACE_DEFAULT uint16 = 50

const RECEIPT_LIMIT_DEFUALT uint8 = 20
