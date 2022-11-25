package bidder

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Agent struct {
	ctx        context.Context
	controller ctr.Controller
	bidder     sgo.PrivateKey
	pcVault    sgo.PublicKey
	bsgReq     brainReqGroup
	internalC  chan<- func(*internal)
}

func Create(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	bidder sgo.PrivateKey,
	pcVaultId sgo.PublicKey,
	pcVault *sgotkn.Account,
	router rtr.Router,
) (Agent, error) {
	internalC := make(chan func(*internal), 10)
	startErrorC := make(chan error, 1)

	ctxC, cancel := context.WithCancel(ctx)

	bsg, bsgReq := createBrainSubGroup()

	go loopInternal(
		ctxC,
		cancel,
		internalC,
		startErrorC,
		rpcClient,
		wsClient,
		bidder,
		pcVaultId,
		pcVault,
		router,
		bsg,
	)

	e1 := Agent{
		ctx:        ctxC,
		internalC:  internalC,
		controller: router.Controller,
		bidder:     bidder,
		pcVault:    pcVaultId,
		bsgReq:     bsgReq,
	}

	return e1, nil
}

func (e1 Agent) CloseSignal() <-chan error {
	doneC := e1.ctx.Done()
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
		return signalC
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
		return signalC
	}
}
