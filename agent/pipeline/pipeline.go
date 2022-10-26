// The proxy receives transactions from bandwidth buyers and forwards those transactions onto the validator via JSON RPC send_tx call.
package pipeline

import (
	"context"
	"errors"
	"net"

	"github.com/solpipe/solpipe-tool/agent/pipeline/admin"
	"github.com/solpipe/solpipe-tool/proxy"
	pxypipe "github.com/solpipe/solpipe-tool/proxy/relay/pipeline"
	pxysvr "github.com/solpipe/solpipe-tool/proxy/server"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	slt "github.com/solpipe/solpipe-tool/state/slot"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Agent struct {
	ctx     context.Context
	cancel  context.CancelFunc
	errorC  chan<- error
	subSlot slt.SlotHome
}

// create a pipeline agent
func Create(
	ctxOutside context.Context,
	args *InitializationArg,
	router rtr.Router,
	pipeline pipe.Pipeline,
) (Agent, error) {
	var err error
	controller := router.Controller
	ctx, cancel := context.WithCancel(ctxOutside)
	pipelineData, err := pipeline.Data()
	if err != nil {
		cancel()
		return Agent{}, err
	}

	if !args.Relay.Admin.PublicKey().Equals(pipelineData.Admin) {
		cancel()
		return Agent{}, errors.New("admin private key does not match")
	}

	err = args.Check()
	if err != nil {
		cancel()
		return Agent{}, err
	}

	adminListener, err := args.Relay.AdminListener()
	if err != nil {
		cancel()
		return Agent{}, err
	}

	errorC := make(chan error, 5)
	config := args.Program

	grpcMainServer := grpc.NewServer()
	grpcAdminServer := grpc.NewServer()
	tpsUpdateErrorC := make(chan error, 1)
	torMgr, err := proxy.SetupTor(ctx, false)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	go loopCloseTor(ctx, torMgr)

	// listen on the tor onion address
	torListener, err := proxy.CreateListener(
		ctx,
		args.Relay.Admin,
		&proxy.ServerConfiguration{},
		torMgr,
	)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	go loopStopOnError(ctx, cancel, tpsUpdateErrorC)
	rpcClient := args.Relay.Rpc()
	wsClient, err := args.Relay.Ws(ctx)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	var signalC <-chan error
	err, signalC = admin.Attach(
		ctx,
		grpcAdminServer,
		router,
		rpcClient,
		wsClient,
		*config.Pipeline,
		args.Admin(),
		args.Program.Settings,
	)
	if err != nil {
		cancel()
		return Agent{}, err
	}

	{
		xC := ctx.Done()
		go func() {
			select {
			case <-xC:
				break
			case err4 := <-signalC:
				errorC <- err4
			}
		}()
	}

	{

		pipelineRelay, err := pxypipe.Create(
			ctx,
			*args.Relay,
			router,
			pipeline,
		)
		if err != nil {
			cancel()
			return Agent{}, err
		}
		err = pxysvr.Attach(ctx, grpcMainServer, router, args.Admin(), pipelineRelay)
		if err != nil {
			cancel()
			return Agent{}, err
		}
	}
	if err != nil {
		errorC <- err
		return Agent{}, err
	}

	go loopClose(ctx, torListener)
	reflection.Register(grpcMainServer)
	go loopListen(grpcMainServer, torListener, errorC)

	go loopClose(ctx, adminListener)
	reflection.Register(grpcAdminServer)
	go loopListen(grpcAdminServer, adminListener, errorC)

	go loopCloseFromError(ctx, cancel, errorC)

	return Agent{
		ctx:     ctx,
		cancel:  cancel,
		errorC:  errorC,
		subSlot: controller.SlotHome(),
	}, nil
}

func loopCloseFromError(ctx context.Context, cancel context.CancelFunc, errorC <-chan error) {
	defer cancel()
	select {
	case <-ctx.Done():
	case <-errorC:
	}
}

func loopCloseTor(ctx context.Context, t *tor.Tor) {
	<-ctx.Done()
	t.Close()
}

func (s Agent) CloseSignal() <-chan struct{} {

	return s.ctx.Done()
}

// make a blocking call to close
func (s Agent) Close() {
	doneC := s.ctx.Done()
	err := s.ctx.Err()
	if err != nil {
		return
	}
	s.cancel()
	<-doneC
	return
}
func loopStopOnError(ctx context.Context, cancel context.CancelFunc, errorC <-chan error) {
	doneC := ctx.Done()
	select {
	case <-errorC:
	case <-doneC:
	}
	cancel()
}

func loopListen(grpcServer *grpc.Server, lis net.Listener, errorC chan<- error) {
	errorC <- grpcServer.Serve(lis)
}

func loopClose(ctx context.Context, lis net.Listener) {
	<-ctx.Done()
	err := lis.Close()
	if err != nil {
		log.Debug(err)
	}
}
