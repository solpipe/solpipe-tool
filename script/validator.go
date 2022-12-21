package script

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

func (e1 *Script) AddValidator(
	controller sgo.PublicKey,
	vote sgo.PrivateKey,
	stake sgo.PublicKey,
	admin sgo.PrivateKey,
) (member sgo.PublicKey, err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}
	e1.AppendKey(vote)
	e1.AppendKey(admin)

	member, _, err = val.ValidatorManagerId(controller, vote.PublicKey())
	if err != nil {
		return
	}

	b := cba.NewAddValidatorInstructionBuilder()

	b.SetControllerAccount(controller)
	b.SetValidatorManagerAccount(member)
	b.SetVoteAccount(vote.PublicKey())
	b.SetStakeAccount(stake)
	b.SetVoteAdminAccount(vote.PublicKey()) // this is just the vote account again used for signing
	b.SetValidatorAdminAccount(admin.PublicKey())
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetRentAccount(sgo.SysVarRentPubkey)

	e1.txBuilder.AddInstruction(b.Build())

	return

}

// the validator will create a receipt account tying itself to a pipeline via the pipeline payout account.
func (e1 *Script) ValidatorSetPipeline(
	controllerId sgo.PublicKey,
	payoutId sgo.PublicKey,
	pipelineId sgo.PublicKey,
	validatorId sgo.PublicKey,
	validatorAdmin sgo.PrivateKey,
) (receiptId sgo.PublicKey, err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}

	receipt, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return
	}
	receiptId = receipt.PublicKey()

	b := cba.NewValidatorSetPayoutInstructionBuilder()
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controllerId)
	b.SetPayoutAccount(payoutId)
	b.SetPipelineAccount(pipelineId)
	b.SetReceiptAccount(receipt.PublicKey())
	e1.AppendKey(receipt)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetValidatorAdminAccount(validatorAdmin.PublicKey())
	e1.AppendKey(validatorAdmin)
	b.SetValidatorManagerAccount(validatorId)

	e1.txBuilder.AddInstruction(b.Build())

	return
}

func (e1 *Script) ValidatorWithdrawReceipt(
	controller ctr.Controller,
	payoutId sgo.PublicKey,
	pipeline pipe.Pipeline,
	validator val.Validator,
	receiptId sgo.PublicKey,
	payer sgo.PrivateKey,
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
	validatorData, err := validator.Data()
	if err != nil {
		return
	}

	b := cba.NewValidatorWithdrawReceiptInstructionBuilder()
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controller.Id())
	b.SetPayoutAccount(payoutId)
	b.SetPcVaultAccount(pipelineData.PcVault)
	b.SetPipelineAccount(pipeline.Id)
	b.SetReceiptAccount(receiptId)
	b.SetTokenProgramAccount(sgo.TokenProgramID)
	b.SetValidatorAdminAccount(validatorData.Admin)
	fundAccountId, err := e1.token_account(payer, validatorData.Admin, controllerData.PcMint)
	if err != nil {
		return
	}
	b.SetValidatorFundAccount(fundAccountId)
	b.SetValidatorManagerAccount(validator.Id)

	e1.txBuilder.AddInstruction(b.Build())
	return nil
}

type ValidatorReceipt struct {
	Receipt sgo.PublicKey
	Payout  sgo.PublicKey
}

// Pipeline creates receipt, sends tx to Validator for signature.
// Use the e1.ExportTx(true) to get the transaction as a raw binary
func (e1 *Script) Deprecate_CreateValidatorReceipt(
	controllerId sgo.PublicKey,
	pipelineId sgo.PublicKey,
	vote sgo.PublicKey,
	pipelineAdmin sgo.PrivateKey,
	validatorAdmin sgo.PublicKey,
	start uint64,
) (tx sgo.Transaction, err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}
	err = errors.New("not implemented yet")

	return
}

// Pipeline sends a signed Receipt.  The Validator appends its signature.
func SignValidatorReceipt(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	pipelineAdmin sgo.PublicKey,
	txData []byte,
	validatorAdmin sgo.PrivateKey,
) (err error) {

	var tx *sgo.Transaction
	tx, err = ParseTransaction(txData)
	if err != nil {
		return
	}

	if !tx.Message.IsSigner(pipelineAdmin) {
		err = errors.New("no pipeline admin signature")
		return
	}
	preLength := len(tx.Signatures)

	tx.Sign(func(pubkey sgo.PublicKey) *sgo.PrivateKey {
		if pubkey.Equals(validatorAdmin.PublicKey()) {
			return &validatorAdmin
		}
		return nil
	})
	postLength := len(tx.Signatures)

	if postLength <= preLength {
		err = errors.New("no length")
		return
	}

	err = SendTx(ctx, rpcClient, wsClient, tx, false)
	if err != nil {
		return
	}

	return

}
