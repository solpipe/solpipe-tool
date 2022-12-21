package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

// clear out all
func (e1 *Script) MultipleClaimRefund(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	payer sgo.PrivateKey,
) error {
	list, err := pipeline.AllClaim()
	if err != nil {
		return err
	}
	for _, c := range list {
		err = e1.ClaimRefund(controller, pipeline, c, payer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e1 *Script) ClaimRefund(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	claim cba.Claim,
	payer sgo.PrivateKey,
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
	_, err = e1.rpc.GetAccountInfo(e1.ctx, userVaultId)
	if err != nil {
		err = e1.CreateTokenAccount(payer, claim.User, cdata.PcMint)
		if err != nil {
			return err
		}
	}
	b.SetUserFundAccount(userVaultId)

	e1.txBuilder.AddInstruction(b.Build())

	return nil
}
