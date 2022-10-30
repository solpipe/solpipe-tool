package validator

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/solpipe/solpipe-tool/proxy/relay"
)

type submitInfo struct {
	ctx    context.Context
	tx     *sgo.Transaction
	errorC chan<- error
	sigC   chan<- sgo.Signature
	bidder sgo.PublicKey
}

func (e1 external) Submit(
	ctx context.Context,
	sender sgo.PublicKey,
	tx *sgo.Transaction,
) (sgo.Signature, error) {

	pubkey, err := relay.GetPeerPubkey(ctx)
	if err != nil {
		return sgo.Signature{}, err
	}
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	sigC := make(chan sgo.Signature, 1)
	si := &submitInfo{ctx: ctx, tx: tx, errorC: errorC, sigC: sigC, bidder: pubkey}

	var sig sgo.Signature

	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.txC <- si:
	}
	if err != nil {
		return sig, err
	}

	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return sig, err
	}
	return <-sigC, nil
}

func (e1 external) Wait(ctx context.Context, signature sgo.Signature) error {
	return errors.New("not implemented yet")
}

// do a json rpc connection
func (in *internal) process(si *submitInfo) {
	if si == nil {
		return
	}
	if si.tx == nil {
		select {
		case si.errorC <- errors.New("blank transaction"):
		default:
		}
		return
	}
	go loopRpcSendTx(si.ctx, si.tx, in.config.Rpc(), si.errorC, si.sigC)
}

func loopRpcSendTx(ctx context.Context, tx *sgo.Transaction, rpcClient *sgorpc.Client, errorC chan<- error, sigC chan<- sgo.Signature) {
	var err error
	var sig sgo.Signature
	sig, err = rpcClient.SendTransactionWithOpts(ctx, tx, sgorpc.TransactionOpts{
		SkipPreflight: true,
	})
	errorC <- err
	if err == nil {
		sigC <- sig
	}
}
