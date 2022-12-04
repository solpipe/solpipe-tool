package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/util"
)

func Crank(
	controller ctr.Controller,
	pipelineId sgo.PublicKey,
	periodId sgo.PublicKey,
	bidId sgo.PublicKey,
	payoutId sgo.PublicKey,
	cranker sgo.PublicKey,
	crankerFunds sgo.PublicKey,
) (instruction sgo.Instruction, err error) {

	builder := cba.NewCrankInstructionBuilder()
	vaultId, _, err := vrs.PipelineVaultId(controller.Version, pipelineId)
	if err != nil {
		return nil, err
	}
	builder.SetControllerAccount(controller.Id())
	builder.SetCrankerAccount(cranker)
	builder.SetCrankerFundAccount(crankerFunds)
	builder.SetPayoutAccount(payoutId)
	builder.SetPipelineAccount(pipelineId)
	builder.SetPcVaultAccount(vaultId)
	builder.SetPeriodsAccount(periodId)
	builder.SetBidsAccount(bidId)
	builder.SetTokenProgramAccount(sgo.TokenProgramID)
	builder.SetClockAccount(sgo.SysVarClockPubkey)

	instruction = builder.Build()
	return
}

func (e1 *Script) Crank(
	controller ctr.Controller,
	pipelineId sgo.PublicKey,
	periodId sgo.PublicKey,
	payoutId sgo.PublicKey,
	cranker sgo.PrivateKey,
	crankerFunds sgo.PublicKey,
) error {
	bidId := util.Zero()
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	instructions, err := Crank(
		controller,
		pipelineId,
		periodId,
		bidId,
		payoutId,
		cranker.PublicKey(),
		crankerFunds,
	)
	if err != nil {
		return err
	}
	e1.txBuilder.AddInstruction(instructions)
	e1.AppendKey(cranker)
	return nil
}
