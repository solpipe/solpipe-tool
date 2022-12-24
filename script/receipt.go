package script

import (
	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
)

type BidReceiptSettings struct {
	Receipt       sgo.PublicKey
	Controller    sgo.PublicKey
	Pipeline      sgo.PublicKey
	PipelineAdmin sgo.PrivateKey
	Payout        sgo.PublicKey
}

func (brs BidReceiptSettings) Update(txHash sgo.Hash) (*cba.UpdateBidReceipt, error) {

	b := cba.NewUpdateBidReceiptInstructionBuilder()

	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(brs.Controller)
	b.SetPayoutAccount(brs.Payout)
	b.SetPipelineAccount(brs.Pipeline)
	b.SetPipelineAdminAccount(brs.PipelineAdmin.PublicKey())
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
	b.SetValidatorManagerAccount(brs.ValidatorMember)

	var d [sgo.PublicKeyLength]byte
	copy(d[:], sgo.PublicKey(txHash).Bytes())
	b.SetLastTx(d)

	return b, nil
}
