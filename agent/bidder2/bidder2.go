package bidder2

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Agent struct {
	ctx       context.Context
	cancel    context.CancelFunc
	router    rtr.Router
	bidder    sgo.PrivateKey
	pcVault   sgo.PublicKey
	internalC chan<- func(*internal)
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
	ctxC, cancel := context.WithCancel(ctx)
	internalC := make(chan func(*internal))
	go loopInternal(ctxC, cancel, internalC, router)

	return Agent{
		ctx: ctxC, cancel: cancel,
		router:    router,
		bidder:    bidder,
		pcVault:   pcVaultId,
		internalC: internalC,
	}, nil
}
