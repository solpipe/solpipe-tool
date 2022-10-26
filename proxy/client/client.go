package client

import (
	"context"
	"errors"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	spt "github.com/solpipe/solpipe-tool/script"
	sgo "github.com/SolmateDev/solana-go"
	"google.golang.org/grpc"
)

type Client struct {
	ctx             context.Context
	internalC       chan<- func(*internal)
	Cancel          context.CancelFunc
	tc              pbj.TransactionClient
	sender          sgo.PrivateKey
	receiver        sgo.PublicKey
	pipelineSetting spt.ReceiptSettings
	bidderSetting   spt.BidReceiptSettings
	isBidder        bool
}

// Use this client to send transactions to either the pipeline/validator.
// Set either bidder or pipeline settings.  Leave one as nil.  The one that has settings set will be the one to which transactions are sent.
// The receipts differ between those going to pipelines and those going to validators.
// Handle the shutdown of the grpc connection separately.
func Create(
	ctx context.Context,
	conn *grpc.ClientConn,
	tx *sgo.Transaction,
	sender sgo.PrivateKey,
	receiver sgo.PublicKey,
	bidder *spt.BidReceiptSettings,
	pipeline *spt.ReceiptSettings,
	script *spt.Script,
) (Client, error) {

	tc := pbj.NewTransactionClient(conn)

	updateSub, err := tc.Update(ctx)
	if err != nil {
		return Client{}, err
	}
	err = updateSub.Send(&pbj.UpdateReceipt{
		Data: &pbj.UpdateReceipt_Setup{
			Setup: &pbj.Setup{
				Sender:   sender.PublicKey().Bytes(),
				Receiver: receiver.Bytes(),
			},
		},
	})
	if err != nil {

		return Client{}, err
	}
	internalC := make(chan func(*internal), 10)
	ctx2, cancel := context.WithCancel(ctx)
	go loopInternal(ctx2, cancel, internalC, tc, sender, receiver, updateSub, script)
	var b spt.BidReceiptSettings
	var p spt.ReceiptSettings
	var isBidder bool
	if bidder != nil {
		isBidder = true
		b = *bidder
	} else {
		isBidder = false
		p = *pipeline
	}
	c := Client{
		ctx:             ctx2,
		internalC:       internalC,
		Cancel:          cancel,
		tc:              tc,
		sender:          sender,
		receiver:        receiver,
		bidderSetting:   b,
		pipelineSetting: p,
		isBidder:        isBidder,
	}

	return c, nil
}

func (e1 Client) send_cb(ctx context.Context, cb func(in *internal)) error {
	doneC := ctx.Done()
	err := ctx.Err()
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		return errors.New("canceled")
	case e1.internalC <- cb:
		return nil
	}
}
