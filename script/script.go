package script

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	bin "github.com/gagliardetto/binary"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type Configuration struct {
	Version vrs.CbaVersion `json:"version"`
}

type Script struct {
	ctx context.Context
	rpc *sgorpc.Client
	ws  *sgows.Client
	//Controller ctr.Controller
	txBuilder *sgo.TransactionBuilder
	keyMap    map[string]sgo.PrivateKey
	config    *Configuration
}

func Create(ctx context.Context, config *Configuration, rpcClient *sgorpc.Client, wsClient *sgows.Client) (*Script, error) {
	if config == nil {
		return nil, errors.New("no config")
	}

	e1 := &Script{
		ctx: ctx, rpc: rpcClient, ws: wsClient,
		config: config,
	}

	return e1, nil
}

func (e1 *Script) AppendKey(key sgo.PrivateKey) {
	e1.keyMap[key.PublicKey().String()] = key
}

func (e1 *Script) AppendInstruction(instruction sgo.Instruction) error {
	if e1.txBuilder == nil {
		return errors.New("blank tx builder")
	}
	e1.txBuilder.AddInstruction(instruction)
	return nil
}

func (e1 *Script) SetTx(payer sgo.PrivateKey) error {
	if e1.txBuilder != nil {
		return errors.New("tx builder already started")
	}
	e1.txBuilder = sgo.NewTransactionBuilder()
	e1.txBuilder.SetFeePayer(payer.PublicKey())
	e1.keyMap = make(map[string]sgo.PrivateKey)
	e1.AppendKey(payer)

	return nil
}

func (e1 *Script) WaitForAccountChange(account sgo.PublicKey) (resultC <-chan WaitResult) {
	ansC := make(chan WaitResult, 1)
	resultC = ansC
	sub, err := e1.ws.AccountSubscribe(account, sgorpc.CommitmentFinalized)
	if err != nil {
		ansC <- WaitResult{Err: err}
		return
	}
	go loopWaitAccount(e1.ctx, sub, ansC)
	return
}

type WaitResult struct {
	HasClosed bool
	Data      []byte
	Err       error
}

func loopWaitAccount(
	ctx context.Context,
	sub *sgows.AccountSubscription,
	resultC chan<- WaitResult,
) {
	defer sub.Unsubscribe()
	doneC := ctx.Done()
	streamC := sub.RecvStream()
	errorC := sub.RecvErr()
	var err error
	var lamports uint64
	var data []byte
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	case d := <-streamC:
		x, ok := d.(*sgows.AccountResult)
		if !ok {
			err = errors.New("bad account result")
		} else {
			lamports = x.Value.Lamports
			data = x.Value.Account.Data.GetBinary()
		}
	}
	if err != nil {
		resultC <- WaitResult{
			Err: err,
		}
		return
	}
	if 0 < lamports {
		resultC <- WaitResult{
			Err:       nil,
			HasClosed: false,
			Data:      data,
		}
	} else {
		resultC <- WaitResult{
			Err:       nil,
			HasClosed: true,
		}
	}
}

func ParseTransaction(data []byte) (tx *sgo.Transaction, err error) {
	tx = new(sgo.Transaction)
	err = bin.NewBinDecoder(data).Decode(tx)
	return
}

func (e1 *Script) sendTxDirect(tx *sgo.Transaction, simulate bool) error {
	return SendTx(e1.ctx, e1.rpc, e1.ws, tx, simulate)
}

func SendTx(ctx context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client, tx *sgo.Transaction, simulate bool) error {
	var err error
	var sig sgo.Signature

	sig, err = rpcClient.SendTransactionWithOpts(ctx, tx, sgorpc.TransactionOpts{SkipPreflight: !simulate})
	if err != nil {
		return err
	}
	err = WaitSig(ctx, wsClient, sig)
	if err != nil {
		return err
	}
	return nil
}

// Instead of sending the transaction via RPC, just export the transaction as a binary
func (e1 *Script) ExportTx(ignoreSigError bool) ([]byte, error) {
	if e1.txBuilder == nil {
		return nil, errors.New("no tx builder")
	}
	var err error
	var rh *sgorpc.GetLatestBlockhashResult
	e1.txBuilder.SetRecentBlockHash(rh.Value.Blockhash)
	var tx *sgo.Transaction
	tx, err = e1.txBuilder.Build()
	if err != nil {
		return nil, err
	}
	_, err = tx.Sign(func(p sgo.PublicKey) *sgo.PrivateKey {
		x, present := e1.keyMap[p.String()]
		if present {
			return &x
		}
		return nil
	})
	if !ignoreSigError && err != nil {
		return nil, err
	} else if err != nil {
		err = nil
	}
	return tx.MarshalBinary()
}

func (e1 *Script) SetBlockHash() (err error) {
	var rh *sgorpc.GetLatestBlockhashResult
	rh, err = e1.rpc.GetLatestBlockhash(e1.ctx, sgorpc.CommitmentFinalized)
	if err != nil {
		return
	}
	e1.txBuilder.SetRecentBlockHash(rh.Value.Blockhash)
	return
}

// compile the transaction and send it to a validator
func (e1 *Script) FinishTx(simulate bool) (err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}

	err = e1.SetBlockHash()
	if err != nil {
		return
	}

	var tx *sgo.Transaction
	tx, err = e1.txBuilder.Build()
	if err != nil {
		return
	}
	_, err = tx.Sign(func(p sgo.PublicKey) *sgo.PrivateKey {
		x, present := e1.keyMap[p.String()]
		if present {
			return &x
		}
		return nil
	})
	if err != nil {
		return
	}
	e1.txBuilder = nil
	e1.keyMap = nil
	err = e1.sendTxDirect(tx, simulate)
	if err != nil {
		return
	}

	return
}
