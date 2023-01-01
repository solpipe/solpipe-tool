package pipeline

import (
	"context"

	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	pbj.UnimplementedTransactionServer
	ctx                     context.Context
	internalC               chan<- func(*internal)
	Cancel                  context.CancelFunc
	network                 ntk.Network
	pipeline                pipe.Pipeline
	requestTxSubmitChannelC chan<- requestForSubmitChannel
}

// Submit transactions from bidders and relay those transactions to validators.
func Create(
	ctx context.Context,
	config relay.Configuration,
	router rtr.Router,
	pipeline pipe.Pipeline,
	torMgr *tor.Tor,
) (relay.Relay, error) {
	log.Debugf("starting relay for pipeline=%s", pipeline.Id.String())
	ctx2, cancel := context.WithCancel(ctx)
	internalC := make(chan func(*internal), 10)
	txSubmitC := make(chan requestForSubmitChannel)
	slotHome := router.Controller.SlotHome()

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
		router,
		pipeline,
		config,
	)
	e1 := external{
		ctx:                     ctx,
		internalC:               internalC,
		Cancel:                  cancel,
		network:                 router.Network,
		pipeline:                pipeline,
		requestTxSubmitChannelC: txSubmitC,
	}
	return e1, nil
}
