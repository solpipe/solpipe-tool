package test

import (
	"context"
	"errors"

	"github.com/solpipe/solpipe-tool/script"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type MintAirdropper struct {
	Id       sgo.PublicKey
	requestC chan<- mintRequest
}

func (m MintAirdropper) Send(ctx context.Context, dst sgo.PublicKey, amount uint64) <-chan error {
	errorC := make(chan error, 1)
	m.requestC <- mintRequest{
		destination: dst, amount: amount, ctx: ctx, errorC: errorC,
	}
	return errorC
}

type internal struct {
	ctx       context.Context
	rpc       *sgorpc.Client
	ws        *sgows.Client
	mint      sgo.PublicKey
	authority sgo.PrivateKey
}

type mintRequest struct {
	destination sgo.PublicKey
	amount      uint64
	ctx         context.Context
	errorC      chan<- error
}

func CreateMintAirdropper(ctx context.Context, rpcUrl string, wsUrl string, version vrs.CbaVersion, mr *script.MintResult) (MintAirdropper, error) {
	requestC := make(chan mintRequest, 10)
	if mr == nil {
		return MintAirdropper{}, errors.New("no mint result")
	}
	if mr.Id == nil {
		return MintAirdropper{}, errors.New("no mint id")
	}
	if mr.Authority == nil {
		return MintAirdropper{}, errors.New("no mint authority")
	}

	rpcClient := sgorpc.New(rpcUrl)

	_, err := rpcClient.RequestAirdrop(ctx, mr.Authority.PublicKey(), 10*sgo.LAMPORTS_PER_SOL, sgorpc.CommitmentConfirmed)
	if err != nil {
		return MintAirdropper{}, err
	}

	wsClient, err := sgows.Connect(ctx, wsUrl)
	if err != nil {
		return MintAirdropper{}, err
	}

	go loopInternal(ctx, rpcClient, wsClient, version, mr, requestC)

	return MintAirdropper{
		requestC: requestC,
		Id:       *mr.Id,
	}, nil

}

func loopInternal(ctx context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client, version vrs.CbaVersion, mr *script.MintResult, requestC <-chan mintRequest) {

	in := new(internal)
	in.ctx = ctx
	in.rpc = rpcClient
	in.ws = wsClient
	in.mint = *mr.Id
	in.authority = *mr.Authority

	doneC := ctx.Done()
out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-requestC:
			s1, err := script.Create(in.ctx, &script.Configuration{Version: version}, in.rpc, in.ws)
			if err != nil {
				req.errorC <- err
			} else {
				err = s1.SetTx(in.authority)
				if err != nil {
					req.errorC <- err
				} else {
					go loopMintTo(s1, req, in.mint, in.authority)
				}
			}
		}
	}
}

func loopMintTo(s1 *script.Script, req mintRequest, mint sgo.PublicKey, authority sgo.PrivateKey) {
	var err error

	err = s1.MintIssue(
		&script.MintResult{Id: &mint, Authority: &authority},
		req.destination,
		req.amount,
	)
	if err != nil {
		req.errorC <- err
		return
	}

	req.errorC <- s1.FinishTx(false)
}
