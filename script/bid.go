package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
)

func (e1 *Script) CloseBids(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	payout pyt.Payout,
	admin sgo.PrivateKey,
) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	data, err := payout.Data()
	if err != nil {
		return err
	}
	b := cba.NewCloseBidsInstructionBuilder()
	b.SetBidsAccount(data.Bids)
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controller.Id())
	b.SetPayoutAccount(payout.Id)
	b.SetPipelineAccount(pipeline.Id)
	b.SetPipelineAdminAccount(admin.PublicKey())
	e1.AppendKey(admin)

	e1.txBuilder.AddInstruction(b.Build())

	return nil
}

func (e1 *Script) bidListSize(bidSpace uint16) uint64 {

	return util.STRUCT_SIZE_BID_LIST_HEADER + uint64(bidSpace)*util.STRUCT_BID_SINGLE
}

func (e1 *Script) Bid(
	controller ctr.Controller,
	bid cba.Bid,
	pipeline pipe.Pipeline,
	payout pyt.Payout,
	bidAdmin sgo.PrivateKey,
	bidFund sgo.PublicKey,
) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	pipeData, err := pipeline.Data()
	if err != nil {
		return err
	}
	payData, err := payout.Data()
	if err != nil {
		return err
	}
	b := cba.NewInsertBidInstructionBuilder()
	b.SetBidsAccount(payData.Bids)
	b.SetControllerAccount(controller.Id())
	b.SetPayoutAccount(payout.Id)
	b.SetPcVaultAccount(pipeData.PcVault)
	b.SetPeriodsAccount(pipeData.Periods)
	b.SetPipelineAccount(pipeline.Id)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	b.SetUserAccount(bidAdmin.PublicKey())
	e1.AppendKey(bidAdmin)
	b.SetUserFundAccount(bidFund)

	b.SetBid(bid)
	if !bid.User.Equals(bidAdmin.PublicKey()) {
		return errors.New("bid admin private key does not match user in bid")
	}

	e1.txBuilder.AddInstruction(b.Build())
	return nil
}
