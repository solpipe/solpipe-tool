package cranker

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Cranker struct {
	ctx       context.Context
	internalC chan<- func(*internal)
	router    rtr.Router
	cancel    context.CancelFunc
}

func Create(
	ctx context.Context,
	config *relay.Configuration,
	balanceThreshold uint64,
	router rtr.Router,
) (Cranker, error) {
	pcVault, err := SetupPcVault(ctx, router, config)
	if err != nil {
		return Cranker{}, err
	}

	startErrorC := make(chan error, 1)
	internalC := make(chan func(*internal), 10)
	ctx2, cancel := context.WithCancel(ctx)
	go loopInternal(
		ctx2,
		cancel,
		internalC,
		startErrorC,
		config,
		pcVault,
		balanceThreshold,
		router,
	)
	err = <-startErrorC
	if err != nil {
		return Cranker{}, err
	}

	return Cranker{
		ctx:       ctx,
		internalC: internalC,
		router:    router,
		cancel:    cancel,
	}, nil
}

func (e1 Cranker) Close() <-chan error {
	signalC := e1.CloseSignal()
	e1.cancel()
	return signalC
}

func (e1 Cranker) CloseSignal() <-chan error {
	doneC := e1.ctx.Done()
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}

	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	if err != nil {
		signalC <- err
		return signalC
	}
	return signalC
}

func pcVaultId(pcVault *sgotkn.Account) (sgo.PublicKey, error) {
	if pcVault == nil {
		return sgo.PublicKey{}, errors.New("no pc vault")
	}
	id, _, err := sgo.FindAssociatedTokenAddress(pcVault.Owner, pcVault.Mint)
	return id, err
}
