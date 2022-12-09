package validator

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
)

func loopDeleteReceipt(
	ctxParent context.Context,
	ctxChild context.Context,
	deleteC chan<- sgo.PublicKey,
	payoutId sgo.PublicKey,
) {
	parentC := ctxParent.Done()
	childC := ctxChild.Done()
	select {
	case <-parentC:
		// we are turning off completely
	case <-childC:
		select {
		case <-parentC:
		case deleteC <- payoutId:
		}
	}
}
