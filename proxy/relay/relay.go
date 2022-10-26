package relay

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
)

// send transactions and apply rate limiting
type Relay interface {
	// send a transaction
	Submit(ctx context.Context, sender sgo.PublicKey, tx *sgo.Transaction) (sgo.Signature, error)
	// set the transactions per second for a given sender (does not apply to Validator relay)
	//AdjustRate(ctx context.Context, sender sgo.PublicKey, newRate float64) error
	// wait for the transaction to show up in a block
	Wait(ctx context.Context, signature sgo.Signature) error
}
