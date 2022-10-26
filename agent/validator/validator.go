package validator

import (
	"context"
	"errors"
	"net"
	"time"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/proxy"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	pxyval "github.com/solpipe/solpipe-tool/proxy/relay/validator"
	pxysvr "github.com/solpipe/solpipe-tool/proxy/server"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	val "github.com/solpipe/solpipe-tool/state/validator"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Agent struct {
	ctx        context.Context
	Cancel     context.CancelFunc
	internalC  chan<- func(*internal)
	controller ctr.Controller
	router     rtr.Router
	Vote       sgo.PublicKey
	Validator  val.Validator
}

type Configuration struct {
	Version vrs.CbaVersion
	Admin   sgo.PrivateKey
}

type ListenResult struct {
	AgentC <-chan Agent
	ErrorC <-chan error
}

func CreateFromListener(
	ctx context.Context,
	config rly.Configuration,
	router rtr.Router,
	vote sgo.PublicKey,
	timeout time.Duration,
) (l ListenResult) {
	ctxShort, cancelShort := context.WithTimeout(ctx, timeout)

	log.Debugf("listening for vote=%s", vote.String())
	sh := router.ObjectOnValidator(func(vwd rtr.ValidatorWithData) bool {
		log.Debugf("checking for vote=%s", vwd.Data.Vote.String())
		if vwd.Data.Vote.Equals(vote) {
			return true
		} else {
			return false
		}
	})
	agentC := make(chan Agent, 1)
	errorC := make(chan error, 1)
	l = ListenResult{AgentC: agentC, ErrorC: errorC}

	rpcClient := config.Rpc()
	wsClient, err := config.Ws(ctxShort)
	if err != nil {
		errorC <- err
		cancelShort()
		return l
	}

	go loopListener(
		ctx,
		ctxShort,
		cancelShort,
		sh,
		agentC,
		errorC,
		config,
		rpcClient,
		wsClient,
		router,
	)

	return
}

func loopListener(
	ctx context.Context,
	ctxShort context.Context,
	cancelShort context.CancelFunc,
	sh sub.Subscription[rtr.ValidatorWithData],
	agentC chan<- Agent,
	errorC chan<- error,
	config rly.Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	router rtr.Router,
) {
	defer cancelShort()
	defer sh.Unsubscribe()

	var err error
	var v val.Validator
	select {
	case <-ctxShort.Done():
		err = errors.New("timed out")
	case err = <-sh.ErrorC:
	case vwd := <-sh.StreamC:
		v = vwd.V
	}

	if err != nil {
		errorC <- err
		return
	}
	a, err := Create(ctx, config, router, v)
	errorC <- err
	if err != nil {
		return
	}
	agentC <- a
	return
}

func Create(
	ctx context.Context,
	config rly.Configuration,
	router rtr.Router,
	validator val.Validator,
) (agent Agent, err error) {
	var data cba.ValidatorMember
	data, err = validator.Data()
	if err != nil {
		return
	}
	controller := router.Controller

	ctx2, cancel := context.WithCancel(ctx)
	t1, err := proxy.SetupTor(ctx2, true)
	if err != nil {
		cancel()
		return
	}
	adminListener, err := config.AdminListener()
	if err != nil {
		cancel()
		return
	}
	mainListener, err := proxy.CreateListener(ctx2, config.Admin, nil, t1)
	if err != nil {
		cancel()
		return
	}
	rpcClient := config.Rpc()
	wsClient, err := config.Ws(ctx2)
	if err != nil {
		cancel()
		return
	}
	//rpcClient *sgorpc.Client, wsClient *sgows.Client
	grpcMainServer := grpc.NewServer()
	grpcAdminServer := grpc.NewServer()
	script, err := config.ScriptBuilder(ctx)
	if err != nil {
		cancel()
		return
	}

	var relay rly.Relay
	relay, err = pxyval.Create(ctx, validator, router.Network, rly.Configuration{
		Version: config.Version,
		Admin:   config.Admin,
	})
	if err != nil {
		cancel()
		return
	}

	// server to process Transaction Submissions
	err = pxysvr.Attach(ctx, grpcMainServer, router, config.Admin, relay)
	if err != nil {
		cancel()
		return
	}
	serverErrorC := make(chan error, 2)
	internalC := make(chan func(*internal), 10)

	reflection.Register(grpcMainServer)
	go loopGrpcListen(ctx2, mainListener, grpcMainServer, serverErrorC)
	go loopGrpcShutdown(ctx2, t1, mainListener, grpcMainServer)

	go loopInternal(
		ctx2,
		cancel,
		serverErrorC,
		internalC,
		config,
		rpcClient,
		wsClient,
		script,
		router,
		validator,
	)
	agent = Agent{
		ctx:        ctx2,
		Cancel:     cancel,
		internalC:  internalC,
		controller: controller,
		router:     router,
		Vote:       data.Vote,
		Validator:  validator,
	}

	err = agent.AttachAdmin(grpcAdminServer)
	if err != nil {
		cancel()
		return
	}
	reflection.Register(grpcAdminServer)
	go loopGrpcListen(ctx2, adminListener, grpcAdminServer, serverErrorC)
	go loopGrpcShutdown(ctx2, nil, adminListener, grpcAdminServer)

	return
}

func loopGrpcListen(ctx context.Context, l net.Listener, s *grpc.Server, errorC chan<- error) {
	doneC := ctx.Done()
	select {
	case <-doneC:
	case errorC <- s.Serve(l):
	}
}

func loopGrpcShutdown(ctx context.Context, t1 *tor.Tor, l net.Listener, s *grpc.Server) {
	<-ctx.Done()
	s.GracefulStop()
	l.Close()
	if t1 != nil {
		t1.Close()
	}

}

func (e1 Agent) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}
