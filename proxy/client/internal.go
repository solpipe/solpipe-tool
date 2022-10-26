package client

import (
	"context"
	"errors"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	spt "github.com/solpipe/solpipe-tool/script"
	sgo "github.com/SolmateDev/solana-go"
	bin "github.com/gagliardetto/binary"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	tc               pbj.TransactionClient
	sender           sgo.PrivateKey
	receiver         sgo.PublicKey
	updateSub        pbj.Transaction_UpdateClient
	currentTx        *sgo.Transaction
	txCount          uint32
	receipt          sgo.PublicKey
	script           *spt.Script
}

func loopInternal(ctx context.Context, cancel context.CancelFunc, internalC <-chan func(*internal), tc pbj.TransactionClient, sender sgo.PrivateKey, receiver sgo.PublicKey, updateSub pbj.Transaction_UpdateClient, script *spt.Script) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()
	txC := make(chan *sgo.Transaction, 10)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.tc = tc
	in.sender = sender
	in.receiver = receiver
	in.updateSub = updateSub
	in.currentTx = nil
	in.txCount = 0
	in.script = script

	go loopUpdateRead(in.ctx, in.errorC, txC, updateSub)
out:
	for {
		select {
		case replyTx := <-txC:
			err = in.tx_check(replyTx)
			if err != nil {
				break out
			}
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	in.finish(err)
}

func (in *internal) tx_check(replyTx *sgo.Transaction) error {
	// TODO: check transaction
	return nil
}

func loopUpdateRead(ctx context.Context, errorC chan<- error, txC chan<- *sgo.Transaction, updateSub pbj.Transaction_UpdateClient) {
	doneC := ctx.Done()
	var msg *pbj.UpdateReceipt
	var err error
	var r *pbj.UpdateReceipt_Receipt
	var ok bool
	var tx *sgo.Transaction
out:
	for {
		msg, err = updateSub.Recv()
		if err != nil {
			break out
		}
		d := msg.GetData()
		if d == nil {
			err = errors.New("blank data")
			break out
		}

		switch d.(type) {
		case *pbj.UpdateReceipt_Receipt:
			r, ok = d.(*pbj.UpdateReceipt_Receipt)
			if !ok {
				err = errors.New("unknown type")
				break out
			}
		case *pbj.UpdateReceipt_Setup:
			err = errors.New("should not be receiving setup")
			break out
		default:
			err = errors.New("unknown type")
			break out
		}
		tx, err = sgo.TransactionFromDecoder(bin.NewBorshDecoder(r.Receipt))
		if err != nil {
			break out
		}
		select {
		case <-doneC:
			err = errors.New("canceled")
		case txC <- tx:
		}
	}
	select {
	case errorC <- err:
	default:
	}
}

func (in *internal) finish(err error) {
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (e1 Client) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}
