package script

import (
	"errors"

	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	"github.com/solpipe/solpipe-tool/state/pipeline"
	sgo "github.com/SolmateDev/solana-go"
)

// close the payout account after 100 slots since the finish.
// return SOL to pipeline admin.  send controll revenue to controller pcvault.
func (e1 *Script) ClosePayout(
	controller ctr.Controller,
	pipeline pipeline.Pipeline,
	payout pyt.Payout,
	admin sgo.PrivateKey,
) (err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}

	controllerData, err := controller.Data()
	if err != nil {
		return
	}
	pipelineData, err := pipeline.Data()
	if err != nil {
		return
	}

	b := cba.NewClosePayoutInstructionBuilder()
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controller.Id())
	b.SetControllerPcVaultAccount(controllerData.PcVault)
	b.SetPayoutAccount(payout.Id)
	b.SetPipelineAccount(pipeline.Id)
	b.SetPipelineAdminAccount(admin.PublicKey())
	e1.AppendKey(admin)
	b.SetPipelinePcVaultAccount(pipelineData.PcVault)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	e1.txBuilder.AddInstruction(b.Build())
	return nil
}
