package bidder

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
)

func (e1 Agent) SendTx(
	ctx context.Context,
	tx *sgo.Transaction,
) error {
	// make sure the bidder signature is appended or else the transaction will be rejected
	if !tx.IsSigner(e1.bidder.PublicKey()) {
		messageContent, err := tx.Message.MarshalBinary()
		if err != nil {
			return err
		}
		sig, err := e1.bidder.Sign(messageContent)
		if err != nil {
			return err
		}
		tx.Signatures = append(tx.Signatures, sig)
	}

	return errors.New("not implemented yet")
}
