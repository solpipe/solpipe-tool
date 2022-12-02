package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

func (e1 *Script) ClaimRefund(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	claim cba.Claim,
) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	cdata, err := controller.Data()
	if err != nil {
		return err
	}
	pdata, err := pipeline.Data()
	if err != nil {
		return err
	}
	b := cba.NewClaimRefundInstructionBuilder()
	b.SetControllerAccount(controller.Id())
	b.SetPipelineAccount(pipeline.Id)
	b.SetPipelineVaultAccount(pdata.PcVault)
	b.SetRefundsAccount(pdata.Refunds)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	userVaultId, _, err := sgo.FindAssociatedTokenAddress(claim.User, cdata.PcMint)
	if err != nil {
		return err
	}
	b.SetUserFundAccount(userVaultId)

	e1.txBuilder.AddInstruction(b.Build())

	return nil
}
