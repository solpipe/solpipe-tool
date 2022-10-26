package script

import (
	cba "github.com/solpipe/cba"
	sgo "github.com/SolmateDev/solana-go"
)

type BidReceiptSettings struct {
	Receipt       sgo.PublicKey
	Controller    sgo.PublicKey
	Pipeline      sgo.PublicKey
	PipelineAdmin sgo.PublicKey
	Payout        sgo.PublicKey
	Bidder        sgo.PrivateKey
}

func (brs BidReceiptSettings) Update(txHash sgo.Hash) (*cba.UpdateBidReceipt, error) {

	b := cba.NewUpdateBidReceiptInstructionBuilder()

	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(brs.Controller)
	b.SetPayoutAccount(brs.Payout)
	b.SetPipelineAccount(brs.Pipeline)
	b.SetBidderAccount(brs.Bidder.PublicKey())
	b.SetBidReceiptAccount(brs.Receipt)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetSystemProgramAccount(sgo.SystemProgramID)

	var d [sgo.PublicKeyLength]byte
	copy(d[:], sgo.PublicKey(txHash).Bytes())
	b.SetLastTx(d)

	return b, nil
}

type ReceiptSettings struct {
	Receipt         sgo.PublicKey
	Controller      sgo.PublicKey
	Pipeline        sgo.PublicKey
	PipelineAdmin   sgo.PrivateKey
	Payout          sgo.PublicKey
	ValidatorMember sgo.PublicKey
	ValidatorAdmin  sgo.PublicKey
}

func (brs ReceiptSettings) Update(txHash sgo.Hash) (*cba.UpdateReceipt, error) {

	b := cba.NewUpdateReceiptInstructionBuilder()

	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(brs.Controller)
	b.SetPayoutAccount(brs.Payout)
	b.SetPipelineAccount(brs.Pipeline)
	b.SetPipelineAdminAccount(brs.PipelineAdmin.PublicKey())
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetValidatorMemberAccount(brs.ValidatorMember)
	b.SetValidatorAdminAccount(brs.ValidatorAdmin)

	var d [sgo.PublicKeyLength]byte
	copy(d[:], sgo.PublicKey(txHash).Bytes())
	b.SetLastTx(d)

	return b, nil
}
