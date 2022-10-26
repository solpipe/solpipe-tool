package server

import (
	"context"
	"errors"
	"io"
	"time"

	cba "github.com/solpipe/cba"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	sgo "github.com/SolmateDev/solana-go"
	bin "github.com/gagliardetto/binary"
)

// receive updates on receipts from the sender; send receipt updates back to the sender
// only one update per sender is allowed
func (e1 external) Update(stream pbj.Transaction_UpdateServer) error {
	ctx := stream.Context()
	// first message tells use who the receiver and writer are
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	data := msg.GetData()
	switch data.(type) {
	case *pbj.UpdateReceipt_Setup:
		x, ok := data.(*pbj.UpdateReceipt_Setup)
		if !ok {
			return errors.New("failed to parse client")
		}
		if x.Setup == nil {
			return errors.New("blank setup")
		}
		if len(x.Setup.Receiver) != sgo.PublicKeyLength {
			return errors.New("blank receiver")
		}
		if len(x.Setup.Sender) != sgo.PublicKeyLength {
			return errors.New("blank sender")
		}
		receiver := sgo.PublicKeyFromBytes(x.Setup.Receiver)
		sender := sgo.PublicKeyFromBytes(x.Setup.Sender)
		sendReceiptUpdateToSenderC, w, r := createUpdatePair(ctx, receiver, sender)

		err = e1.set_stream(sender, stream, r.errorC, sendReceiptUpdateToSenderC)
		if err != nil {
			return err
		}
		go loopUpdateReadFromRemote(ctx, stream, receiver, sender, r)
		return e1.update_write_receipt(stream, sender, w, sendReceiptUpdateToSenderC)
	case *pbj.UpdateReceipt_Receipt:
		return errors.New("did not receive client type first")
	default:
		return errors.New("bad client type")
	}
}

func loopDeleteReceipt(ctx context.Context, deleteC chan<- string, sig sgo.Hash) {
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Minute):
		deleteC <- sig.String()
	}
}

// the sender is sending updates to the receipt; handle the updates
func (e1 external) update_write_receipt(stream pbj.Transaction_UpdateServer, sender sgo.PublicKey, writer updateWriteToRemote, sendReceiptUpdateToSenderC chan<- *sgo.Transaction) error {
	doneC := writer.ctx.Done()

	var err error

	var tx *sgo.Transaction
	var instructionData []byte
	var content []byte
	var sig sgo.Signature
out:
	for {
		select {
		case <-doneC:
			break out
		case tx := <-writer.sendReceiptUpdateToSenderC:
			// send a receipt signed by the admin to the sender
			content, err = tx.Message.MarshalBinary()
			if err != nil {
				break out
			}
			sig, err = e1.admin.Sign(content)
			if err != nil {
				break out
			}
			tx.Signatures = append(tx.Signatures, sig)
			var out []byte
			out, err = tx.MarshalBinary()
			if err != nil {
				break out
			}
			err = stream.Send(&pbj.UpdateReceipt{
				Data: &pbj.UpdateReceipt_Receipt{
					Receipt: out,
				},
			})
			if err != nil {
				break out
			}
		case err = <-writer.errorC:
			// someone has failed to write to the stream to update a receipt
			break out
		case tx = <-writer.updateFromSenderC:
			// tx contains latest receipt information from sender
			if !tx.IsSigner(sender) {
				err = errors.New("transaction not signed by sender")
				break out
			}
			accountList := tx.AccountMetaList()
			updateInstruction := tx.Message.Instructions[0]
			if accountList[updateInstruction.ProgramIDIndex] == nil {
				err = errors.New("program account does not exist")
				break out
			}
			instructionData, err = sgo.Base58.MarshalJSON(updateInstruction.Data)
			if err != nil {
				break
			}
			var instruction *cba.Instruction
			instruction, err = cba.DecodeInstruction(tx.AccountMetaList(), instructionData)
			if err != nil {
				break out
			}
			var pair ReceiptPair
			switch instruction.TypeID {
			case cba.Instruction_UpdateBidReceipt:
				args := new(cba.UpdateBidReceipt)
				err = parse(instruction, args)
				if err != nil {
					break out
				}
				if args.TxSent == nil {
					err = errors.New("blank tx sent")
					break out
				}
				if args.LastTx == nil {
					err = errors.New("blank last tx")
					break out
				}
				pair = pairFromBidReceiptArgs(args)
			case cba.Instruction_UpdateReceipt:
				// pipeline
				args := new(cba.UpdateReceipt)
				err = parse(instruction, args)
				if err != nil {
					break out
				}
				if args.TxSent == nil {
					err = errors.New("blank tx sent")
					break out
				}
				if args.LastTx == nil {
					err = errors.New("blank last tx")
					break out
				}
				pair = pairFromPipelineReceiptArgs(args)
			}
			err = e1.set_receipt(writer.ctx, sender, tx, pair, sendReceiptUpdateToSenderC)
			if err != nil {
				break out
			}
		}
	}
	return err
}

// store the stream for updating receipts to make sure only one such stream exists
func (e1 external) set_stream(sender sgo.PublicKey, stream pbj.Transaction_UpdateServer, streamWriteErrorC chan<- error, sendReceiptUpdateToSenderC chan<- *sgo.Transaction) error {
	errorC := make(chan error, 1)
	err := e1.send_cb(stream.Context(), func(in *internal) {
		_, present := in.updateStream[sender.String()]
		if present {
			errorC <- errors.New("stream already set")
		} else {
			errorC <- nil
			in.updateStream[sender.String()] = &streamInfo{
				errorC:                     streamWriteErrorC,
				sendReceiptUpdateToSenderC: sendReceiptUpdateToSenderC,
			}
			go loopDeleteStream(stream.Context(), in.deleteStreamC, sender)
		}
	})
	if err != nil {
		return err
	}
	err = <-errorC
	if err != nil {
		return err
	}
	return nil
}

type UpdateRequest struct {
	Pair   ReceiptPair
	Sender sgo.PublicKey
}

type ReceiptPair struct {
	LastTx  sgo.Hash
	TxCount uint32
}

func pairFromPipelineReceiptArgs(args *cba.UpdateReceipt) ReceiptPair {
	return ReceiptPair{
		LastTx:  sgo.HashFromBytes(args.LastTx[:]),
		TxCount: *args.TxSent,
	}
}

func pairFromBidReceiptArgs(args *cba.UpdateBidReceipt) ReceiptPair {
	return ReceiptPair{
		LastTx:  sgo.HashFromBytes(args.LastTx[:]),
		TxCount: *args.TxSent,
	}
}

// store the receipt where a Submit() call can find it.  Senders authenticate by pre-sending a Receipt, then sending the transaction to be processed
func (e1 external) set_receipt(ctx context.Context, sender sgo.PublicKey, tx *sgo.Transaction, pair ReceiptPair, sendReceiptUpdateToSenderC chan<- *sgo.Transaction) error {
	doneC := ctx.Done()
	hash := pair.LastTx
	var err error
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		_, present := in.updateReceipt[hash.String()]
		if present {
			return
		}
		in.updateReceipt[hash.String()] = &receiptWithArgs{
			sender:                     sender,
			sendReceiptUpdateToSenderC: sendReceiptUpdateToSenderC,
			pair:                       &pair,
			tx:                         tx,
		}
		go loopDeleteReceipt(in.ctx, in.deleteReceiptC, hash)
	}:
	}
	return err
}

func loopDeleteStream(ctx context.Context, deleteC chan<- string, sender sgo.PublicKey) {
	<-ctx.Done()
	deleteC <- sender.String()
}

func parse(instruction *cba.Instruction, out interface{}) error {
	var err error
	var argsData []byte
	argsData, err = instruction.Data()
	if err != nil {
		return err
	}

	err = bin.NewBorshDecoder(argsData).Decode(out)
	if err != nil {
		return err
	}
	return nil
}

type updateWriteToRemote struct {
	client                     pbj.Setup_Client
	ctx                        context.Context
	receiver                   sgo.PublicKey
	sender                     sgo.PublicKey
	sendReceiptUpdateToSenderC <-chan *sgo.Transaction
	updateFromSenderC          <-chan *sgo.Transaction
	errorC                     <-chan error
}

type updateReadFromRemote struct {
	updateC chan<- *sgo.Transaction
	errorC  chan<- error
}

// user/validator + pipeline as payout
// either bidder as user -> pipeline; or pipeline -> validator
func createUpdatePair(ctx context.Context, receiver sgo.PublicKey, sender sgo.PublicKey) (chan<- *sgo.Transaction, updateWriteToRemote, updateReadFromRemote) {
	updateC := make(chan *sgo.Transaction, 1)
	errorC := make(chan error, 1)
	sendReceiptUpdateToSenderC := make(chan *sgo.Transaction, 1)
	a := updateWriteToRemote{
		ctx:                        ctx,
		receiver:                   receiver,
		sender:                     sender,
		sendReceiptUpdateToSenderC: sendReceiptUpdateToSenderC,
		updateFromSenderC:          updateC,
		errorC:                     errorC,
	}
	b := updateReadFromRemote{
		updateC: updateC,
		errorC:  errorC,
	}
	return sendReceiptUpdateToSenderC, a, b
}

// read from the grpc stream and send the resulting data to a channel for
// e1.update_stream_receipt() to handle
func loopUpdateReadFromRemote(ctx context.Context, stream pbj.Transaction_UpdateServer, receiver sgo.PublicKey, sender sgo.PublicKey, reader updateReadFromRemote) {
	var msg *pbj.UpdateReceipt
	var err error
	var receipt *sgo.Transaction
out:
	for {
		msg, err = stream.Recv()
		if err == io.EOF {
			err = nil
			break out
		} else if err != nil {
			break out
		}

		receipt, err = sgo.TransactionFromDecoder(bin.NewBorshDecoder(msg.GetReceipt()))
		if err != nil {
			break out
		}
		reader.updateC <- receipt
	}
	reader.errorC <- err
}
