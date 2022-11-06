package pipeline

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
)

type submitInfo struct {
	ctx    context.Context
	tx     *sgo.Transaction
	errorC chan<- error
	sigC   chan<- sgo.Signature
}

type requestForSubmitChannel struct {
	sender       sgo.PublicKey
	respC        chan<- chan<- submitInfo
	bidderFoundC chan<- bool
}

// From submit, we want a direct connection to the bidder goroutine.
// We want to bypass the internal select loop
func (e1 external) Submit(
	ctx context.Context,
	sender sgo.PublicKey,
	tx *sgo.Transaction,
) (sgo.Signature, error) {
	doneC := ctx.Done()
	var sig sgo.Signature
	var err error
	bidderFoundC := make(chan bool, 1)
	respC := make(chan chan<- submitInfo, 1)
	select {
	case <-doneC:
		return sig, errors.New("canceled")
	case e1.requestTxSubmitChannelC <- requestForSubmitChannel{
		sender:       sender,
		respC:        respC,
		bidderFoundC: bidderFoundC,
	}:
	}
	var submitC chan<- submitInfo
	select {
	case <-doneC:
		return sig, errors.New("canceled")
	case bidderHasBeenFound := <-bidderFoundC:
		if !bidderHasBeenFound {
			return sig, errors.New("bidder not found")
		}
	}
	submitC = <-respC

	sigC := make(chan sgo.Signature, 1)
	errorC := make(chan error, 1)
	select {
	case <-doneC:
		return sig, errors.New("canceled")
	case submitC <- submitInfo{
		ctx:    ctx,
		tx:     tx,
		errorC: errorC,
		sigC:   sigC,
	}:
	}

	err = <-errorC
	if err != nil {
		return sig, err
	}
	sig = <-sigC
	return sig, nil
}

func (e1 external) Wait(ctx context.Context, signature sgo.Signature) error {
	return errors.New("not implemented yet")
}
