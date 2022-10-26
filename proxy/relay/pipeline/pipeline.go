package pipeline

import (
	"context"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	pbj.UnimplementedTransactionServer
	ctx       context.Context
	internalC chan<- func(*internal)
	Cancel    context.CancelFunc
	network   ntk.Network
	pipeline  pipe.Pipeline
	txSubmitC chan<- requestForSubmitChannel
}

// Submit transactions from bidders and relay those transactions to validators.
func Create(
	ctx context.Context,
	config relay.Configuration,
	router rtr.Router,
	pipeline pipe.Pipeline,
) (relay.Relay, error) {
	network := router.Network
	ctx2, cancel := context.WithCancel(ctx)
	internalC := make(chan func(*internal), 10)
	txSubmitC := make(chan requestForSubmitChannel)
	slotHome := router.Controller.SlotHome()
	torMgr, err := proxy.SetupTor(ctx2, false)
	if err != nil {
		cancel()
		return nil, err
	}
	dialer, err := torMgr.Dialer(ctx2, nil)
	if err != nil {
		cancel()
		return nil, err
	}

	go loopInternal(
		ctx2,
		cancel,
		torMgr,
		dialer,
		internalC,
		txSubmitC,
		slotHome,
		network,
		pipeline,
		config,
	)
	e1 := external{
		ctx:       ctx,
		internalC: internalC,
		Cancel:    cancel,
		network:   network,
		pipeline:  pipeline,
		txSubmitC: txSubmitC,
	}
	return e1, nil
}
