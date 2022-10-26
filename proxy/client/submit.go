package client

import (
	"context"
	"errors"

	cba "github.com/solpipe/cba"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

// Send a transaction.  Before sending the transaction, update the receipt with the latest transaction hash so that the receiver will be able to authenticate the transaction.
func (e1 Client) Submit(ctx context.Context, tx *sgo.Transaction) error {
	doneC := ctx.Done()
	data, err := tx.MarshalBinary()
	if err != nil {
		return err
	}
	txHash, err := util.HashTransaction(tx)
	if err != nil {
		return err
	}
	// authenticate this transaction with the receiver before sending the transaction
	if e1.isBidder {
		err = e1.generate_bid_receipt(ctx, txHash)
	} else {
		err = e1.generate_pipeline_receipt(ctx, txHash)
	}
	if err != nil {
		return err
	}

	stream, err := e1.tc.Submit(ctx, &pbj.Request{
		Tx: data,
	})
	if err != nil {
		return err
	}
	errorC := make(chan error, 1)
	go loopSubmitStream(ctx, errorC, stream)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	return err
}

func (e1 Client) generate_pipeline_receipt(ctx context.Context, txHash sgo.Hash) error {

	b, err := e1.pipelineSetting.Update(txHash)
	if err != nil {
		return err
	}
	s := e1.pipelineSetting
	errorC := make(chan error, 1)
	err = e1.send_cb(ctx, func(in *internal) {
		in.txCount++
		b.SetTxSent(in.txCount)
		errorC <- in.generate_general_receipt(s.PipelineAdmin, b.Build())
	})
	if err != nil {
		return err
	}
	return <-errorC
}

func (e1 Client) generate_bid_receipt(ctx context.Context, txHash sgo.Hash) error {

	b, err := e1.bidderSetting.Update(txHash)
	if err != nil {
		return err
	}
	s := e1.bidderSetting
	errorC := make(chan error, 1)
	err = e1.send_cb(ctx, func(in *internal) {
		in.txCount++
		b.SetTxSent(in.txCount)
		errorC <- in.generate_general_receipt(s.Bidder, b.Build())
	})
	if err != nil {
		return err
	}
	return <-errorC
}

func (in *internal) generate_general_receipt(admin sgo.PrivateKey, instruction *cba.Instruction) error {
	var err error
	in.script.AppendKey(admin)
	err = in.script.SetTx(admin)
	if err != nil {
		return err
	}
	err = in.script.AppendInstruction(instruction)
	if err != nil {
		return err
	}

	err = in.script.SetBlockHash()
	if err != nil {
		return err
	}
	var data []byte
	data, err = in.script.ExportTx(true)
	if err != nil {
		return err
	}
	err = in.updateSub.Send(&pbj.UpdateReceipt{
		Data: &pbj.UpdateReceipt_Receipt{
			Receipt: data,
		},
	})
	if err != nil {
		// need to close client since stream send failed
		in.errorC <- err
		return err
	}
	return nil
}

// read from the stream and return an error when the receiver has indicated the transactino has been processed
func loopSubmitStream(ctx context.Context, errorC chan<- error, stream pbj.Transaction_SubmitClient) {
	var msg *pbj.Response
	var err error
out:
	for {
		msg, err = stream.Recv()
		if err != nil {
			break out
		}
		switch msg.GetStatus() {
		case pbj.Status_NEW:
			log.Debug("tx processing")
		case pbj.Status_FAILED:
			err = errors.New("tx failed")
			break out
		case pbj.Status_FINISHED:
			err = nil
			break out
		}
	}
}
