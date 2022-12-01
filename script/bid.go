package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

func (e1 *Script) Bid(
	controller ctr.Controller,
	bid cba.Bid,
	p pipe.Pipeline,
	payout pyt.Payout,
	bidAdmin sgo.PrivateKey,
	bidFund sgo.PublicKey,
) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	data, err := p.Data()
	if err != nil {
		return err
	}

	b := cba.NewInsertBidInstructionBuilder()
	b.SetBidsAccount(data.Bids)
	b.SetControllerAccount(controller.Id())
	b.SetPayoutAccount(payout.Id)
	b.SetPcVaultAccount(data.PcVault)
	b.SetPeriodsAccount(data.Periods)
	b.SetPipelineAccount(p.Id)
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
