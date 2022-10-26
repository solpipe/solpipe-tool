package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgosys "github.com/SolmateDev/solana-go/programs/system"
)

func (e1 *Script) Transfer(source sgo.PrivateKey, destination sgo.PublicKey, amount uint64) error {
	if e1.txBuilder == nil {
		return errors.New("tx builder is blank")
	}
	b := sgosys.NewTransferInstructionBuilder()
	b.SetFundingAccount(source.PublicKey())
	e1.AppendKey(source)
	b.SetRecipientAccount(destination)
	b.SetLamports(amount)
	e1.txBuilder.AddInstruction(b.Build())
	return nil
}
