package server

import (
	"context"
	"errors"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	bin "github.com/gagliardetto/binary"
)

// sender has sent a transaction and the receiver has received it
func (e1 external) Submit(req *pbj.Request, stream pbj.Transaction_SubmitServer) error {
	ctx := stream.Context()
	doneC := ctx.Done()
	if req == nil {
		return errors.New("blank request")
	}
	txData := req.GetTx()
	if txData == nil {
		return errors.New("blank transaction")
	}

	err := stream.Send(&pbj.Response{
		Status: pbj.Status_NEW,
	})
	if err != nil {
		return err
	}

	tx, err := sgo.TransactionFromDecoder(bin.NewBorshDecoder(txData))
	if err != nil {
		return err
	}
	txHash, err := util.HashTransaction(tx)
	if err != nil {
		return err
	}
	// authorize the connection
	r, err := e1.get_receipt(ctx, txHash)
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case r.sendReceiptUpdateToSenderC <- r.tx:
	}
	if err != nil {
		return err
	}

	sig, err := e1.relay.Submit(ctx, r.sender, r.tx)
	if err != nil {
		return nil
	}
	err = e1.relay.Wait(ctx, sig)
	if err != nil {
		return nil
	}

	return nil
}

// sender is sending a transaction, so look up to see if it has been authenticated
func (e1 external) get_receipt(ctx context.Context, txHash sgo.Hash) (a *receiptWithArgs, err error) {
	doneC := ctx.Done()
	err = ctx.Err()
	if err != nil {
		return
	}

	errorC := make(chan error, 1)
	ansC := make(chan *receiptWithArgs, 1)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		x, present := in.updateReceipt[txHash.String()]
		if present {
			delete(in.updateReceipt, txHash.String())
			errorC <- nil
			ansC <- x
		} else {
			errorC <- errors.New("no matching tx hash")
		}
	}:
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return
	}
	a = <-ansC

	return
}
