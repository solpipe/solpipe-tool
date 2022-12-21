package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
)

func (e1 *Script) Crank(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	payout pyt.Payout,
	cranker sgo.PrivateKey,
) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	controllerData, err := controller.Data()
	if err != nil {
		return err
	}

	pipelineData, err := pipeline.Data()
	if err != nil {
		return err
	}
	payoutData, err := payout.Data()
	if err != nil {
		return err
	}

	crankerFunds, _, err := sgo.FindAssociatedTokenAddress(cranker.PublicKey(), controllerData.PcMint)
	e1.AppendKey(cranker)
	if err != nil {
		return err
	}
	_, err = e1.rpc.GetAccountInfo(e1.ctx, crankerFunds)
	if err != nil {
		err = e1.CreateTokenAccount(cranker, cranker.PublicKey(), controllerData.PcMint)
		if err != nil {
			return err
		}
	}

	b := cba.NewCrankInstructionBuilder()
	b.SetBidsAccount(payoutData.Bids)
	if payoutData.Bids.Equals(util.Zero()) {
		return errors.New("bid account does not exist")
	}
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controller.Id())
	b.SetCrankerAccount(cranker.PublicKey())
	b.SetCrankerFundAccount(crankerFunds)
	b.SetPayoutAccount(payout.Id)
	b.SetPcVaultAccount(pipelineData.PcVault)
	b.SetPipelineAccount(pipeline.Id)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	e1.txBuilder.AddInstruction(b.Build())
	log.Debugf("crank bid payout_data=(%+v)", payoutData)
	e1.AppendKey(cranker)
	return nil
}
