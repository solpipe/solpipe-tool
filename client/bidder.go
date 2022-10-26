package client

import (
	"context"
	"errors"
	"io"
	"time"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	"google.golang.org/grpc"
)

type Client struct {
	auth    *util.Authentication
	conn    *grpc.ClientConn
	user    sgo.PrivateKey
	vault   *sgotkn.Account
	jobWork pbj.TransactionClient
}

func Create(
	ctx context.Context,
	conn *grpc.ClientConn,
	bidderKey sgo.PrivateKey,
	vault *sgotkn.Account,
) *Client {

	ans := new(Client)
	ans.conn = conn
	ans.user = bidderKey
	ans.vault = vault

	return ans
}

func (c *Client) Close() {
	c.conn.Close()
}

// Addd authentication credentials to the http2 headers
func (c *Client) Ctx(ctx context.Context) (context.Context, error) {
	var err error
	renew := false
	if c.auth == nil {
		renew = true
	} else if c.auth.IsExpired() {
		renew = true
	}

	if renew {
		c.auth, err = util.Authenticate(c.user, 30*time.Minute)
		if err != nil {
			return nil, err
		}
	}

	return c.auth.Ctx(ctx), nil
}

func (c *Client) SendTxNoWait(ctx context.Context, serializedTx []byte) error {
	work := c.jobWork
	job, err := work.Submit(c.auth.Ctx(ctx), &pbj.Request{
		Tx: serializedTx,
	})
	if err != nil {
		return err
	}

	msg, err := job.Recv()
	if err != nil {
		return err
	}

	switch msg.Status {
	case pbj.Status_FAILED:
		return errors.New("transaction failed")
	default:
		return errors.New("unknown transaction")
	}

	return nil
}

// send a signed, serialized transaction; this function blocks until the transaction has been processed
func (c *Client) SendTx(ctx context.Context, serializedTx []byte) <-chan error {
	work := c.jobWork

	errorC := make(chan error, 1)

	stream, err := work.Submit(ctx, &pbj.Request{
		Tx: serializedTx,
	})
	if err != nil {
		errorC <- err
		return errorC
	}

	go streamUntilFinished(stream, errorC)
	return errorC
}

func streamUntilFinished(stream pbj.Transaction_SubmitClient, errorC chan<- error) {
	errorC <- streamInside(stream)
}

func streamInside(stream pbj.Transaction_SubmitClient) error {
	var err error
out:
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			err = nil
			break out
		} else if err != nil {
			break out
		}

		switch msg.Status {
		case pbj.Status_NEW:
		case pbj.Status_STARTED:
		case pbj.Status_FINISHED:
			err = nil
			break out
		case pbj.Status_FAILED:
			err = errors.New("transaction failed")
			break out
		}
	}
	return err
}
