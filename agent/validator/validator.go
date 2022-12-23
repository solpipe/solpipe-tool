package validator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/proxy"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	pxyval "github.com/solpipe/solpipe-tool/proxy/relay/validator"
	pxysvr "github.com/solpipe/solpipe-tool/proxy/server"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	val "github.com/solpipe/solpipe-tool/state/validator"
	vrs "github.com/solpipe/solpipe-tool/state/version"
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
	configFilePath string,
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
		configFilePath,
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
	configFilePath string,
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
	a, err := Create(ctx, config, router, v, configFilePath)
	errorC <- err
	if err != nil {
		return
	}
	agentC <- a

}

func Create(
	ctx context.Context,
	config rly.Configuration,
	router rtr.Router,
	validator val.Validator,
	configFilePath string,
) (agent Agent, err error) {
	var data cba.ValidatorManager
	data, err = validator.Data()
	if err != nil {
		return
	}
	controller := router.Controller

	ctxC, cancel := context.WithCancel(ctx)
	t1, err := proxy.SetupTor(ctxC, true)
	if err != nil {
		cancel()
		return
	}
	adminListener, err := config.AdminListener(ctxC)
	if err != nil {
		cancel()
		return
	}

	var grpcServerTor *grpc.Server
	var grpcServerClearNet *grpc.Server
	var torLi *proxy.ListenerInfo
	var clearLi *proxy.ListenerInfo
	grpcServerTor, err = proxy.CreateListener(
		ctx,
		config.Admin,
	)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	torMgr, err := proxy.SetupTor(ctx, false)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	torLi, err = proxy.CreateListenerTor(
		ctx,
		config.Admin,
		torMgr,
	)
	if err != nil {
		cancel()
		return Agent{}, err
	}
	if config.ClearNet != nil {
		grpcServerClearNet, err = proxy.CreateListener(
			ctx,
			config.Admin,
		)
		if err != nil {
			cancel()
			return Agent{}, err
		}

		clearLi, err = proxy.CreateListenerClearNet(
			ctx,
			fmt.Sprintf(":%d", config.ClearNet.Port),
			[]string{"this part is not relavent"},
		)
		if err != nil {
			cancel()
			return Agent{}, err
		}

	}

	rpcClient := config.Rpc()
	wsClient, err := config.Ws(ctxC)
	if err != nil {
		cancel()
		return
	}

	//rpcClient *sgorpc.Client, wsClient *sgows.Client

	script, err := config.ScriptBuilder(ctx)
	if err != nil {
		cancel()
		return
	}
	scriptWrapper := spt.Wrap(ctxC, script)

	var relay rly.Relay
	relay, err = pxyval.Create(ctx, validator, router.Network, config)
	if err != nil {
		cancel()
		return
	}
	internalC := make(chan func(*internal), 10)
	serverErrorC := make(chan error, 4)

	sList := []*grpc.Server{grpcServerTor}
	if grpcServerClearNet != nil {
		sList = append(sList, grpcServerClearNet)
	}

	err = pxysvr.Attach(
		ctx,
		sList,
		router,
		config.Admin,
		relay,
		config.ClearNet,
	)
	if err != nil {
		cancel()
		return
	}
	reflection.Register(grpcServerTor)
	go loopGrpcListen(ctxC, torLi.Listener, grpcServerTor, serverErrorC)
	go loopGrpcShutdown(ctxC, t1, torLi.Listener, grpcServerTor)
	if grpcServerClearNet != nil {
		reflection.Register(grpcServerClearNet)
		go loopGrpcListen(ctxC, clearLi.Listener, grpcServerClearNet, serverErrorC)
		go loopGrpcShutdown(ctxC, t1, clearLi.Listener, grpcServerClearNet)
	}

	log.Debug("entering internal loop for validator agent")
	go loopInternal(
		ctxC,
		cancel,
		serverErrorC,
		internalC,
		config,
		rpcClient,
		wsClient,
		scriptWrapper,
		router,
		validator,
		configFilePath,
	)
	agent = Agent{
		ctx:        ctxC,
		Cancel:     cancel,
		internalC:  internalC,
		controller: controller,
		router:     router,
		Vote:       data.Vote,
		Validator:  validator,
	}
	grpcAdminServer := grpc.NewServer()
	err = agent.AttachAdmin(grpcAdminServer)
	if err != nil {
		cancel()
		return
	}
	reflection.Register(grpcAdminServer)
	go loopGrpcListen(ctxC, adminListener, grpcAdminServer, serverErrorC)
	go loopGrpcShutdown(ctxC, nil, adminListener, grpcAdminServer)

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
