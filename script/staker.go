package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	skr "github.com/solpipe/solpipe-tool/state/staker"
)

func (e1 *Script) AddStaker(
	controllerId sgo.PublicKey,
	stake sgo.PrivateKey,
	admin sgo.PrivateKey,
) error {
	var err error
	if e1.txBuilder == nil {
		return errors.New("no tx builder")
	}
	var managerId sgo.PublicKey
	managerId, _, err = skr.StakerManagerId(controllerId, stake.PublicKey())
	if err != nil {
		return err
	}

	b := cba.NewAddStakerInstructionBuilder()
	b.SetControllerAccount(controllerId)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetStakerAccount(stake.PublicKey())
	e1.AppendKey(stake)
	b.SetStakerManagerAccount(managerId)
	b.SetStakerSignerAccount(admin.PublicKey())
	e1.AppendKey(admin)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	e1.txBuilder.AddInstruction(b.Build())

	return nil
}

func (e1 *Script) AddStakerToReceipt(
	controllerId sgo.PublicKey,
	stake sgo.PublicKey,
	admin sgo.PrivateKey,
	payoutId sgo.PublicKey,
	receiptId sgo.PublicKey,
	validatorManagerId sgo.PublicKey,
) error {
	var err error
	if e1.txBuilder == nil {
		return errors.New("no tx builder")
	}
	var managerId sgo.PublicKey
	managerId, _, err = skr.StakerManagerId(controllerId, stake)
	if err != nil {
		return err
	}
	var stakerReceiptId sgo.PublicKey
	stakerReceiptId, _, err = skr.StakerReceiptId(managerId, stakerReceiptId)
	if err != nil {
		return err
	}

	b := cba.NewAddStakerToReceiptInstructionBuilder()
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controllerId)
	b.SetPayoutAccount(payoutId)
	b.SetReceiptAccount(receiptId)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetStakeAdminAccount(admin.PublicKey())
	e1.AppendKey(admin)
	b.SetStakerAccount(stake)
	b.SetStakerManagerAccount(managerId)
	b.SetStakerReceiptAccount(stakerReceiptId)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetValidatorManagerAccount(validatorManagerId)

	e1.txBuilder.AddInstruction(b.Build())

	return nil
}
