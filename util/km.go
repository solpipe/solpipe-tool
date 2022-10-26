package util

import (
	"context"
	"errors"
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type TxScaffold struct {
	Builder sgo.TransactionBuilder
	m       map[string]sgo.PrivateKey
	ctx     context.Context
	rpc     *sgorpc.Client
	ws      *sgows.Client
}

func CreateScaffold(ctx context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client) *TxScaffold {
	return &TxScaffold{
		Builder: *sgo.NewTransactionBuilder(),
		m:       make(map[string]sgo.PrivateKey),
		ctx:     ctx,
		rpc:     rpcClient,
		ws:      wsClient,
	}
}

func (km *TxScaffold) Add(key sgo.PrivateKey) {
	km.m[key.PublicKey().String()] = key
}

func (km *TxScaffold) Find(p sgo.PublicKey) *sgo.PrivateKey {
	ans, present := km.m[p.String()]
	if present {
		return &ans
	} else {
		return nil
	}
}

func (ts *TxScaffold) Send() (*sgo.Transaction, *sgo.Signature, error) {
	{
		rh, err := ts.rpc.GetLatestBlockhash(ts.ctx, sgorpc.CommitmentFinalized)
		if err != nil {
			return nil, nil, err
		}
		ts.Builder.SetRecentBlockHash(rh.Value.Blockhash)
	}

	tx, err := ts.Builder.Build()
	if err != nil {
		return nil, nil, err
	}
	_, err = tx.Sign(ts.Find)
	if err != nil {
		return nil, nil, err
	}

	sig, err := ts.rpc.SendTransactionWithOpts(ts.ctx, tx, sgorpc.TransactionOpts{})
	if err != nil {
		return nil, nil, err
	}
	sub, err := ts.ws.SignatureSubscribe(sig, sgorpc.CommitmentConfirmed)
	if err != nil {
		//log.Debug(err)
		return nil, nil, err
	}

	doneC := ts.ctx.Done()
	streamC := sub.RecvStream()

	select {
	case <-doneC:
		err = errors.New("time out")
	case d := <-streamC:
		result, ok := d.(*sgows.SignatureResult)
		if ok {
			if result.Value.Err != nil {
				err = fmt.Errorf("%s", result.Value.Err)
			}
		} else {
			err = errors.New("failed to deserialize")
		}
	}
	if err != nil {
		return nil, nil, err
	}
	return tx, &sig, nil
}
