package web

import (
	"context"

	"net/http"
	"net/url"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/ds/ts/lite"
	prc "github.com/solpipe/solpipe-tool/state/pricing"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	ctx         context.Context
	closeErrorC chan<- error
	healthC     chan func(*healthInternal)
	router      rtr.Router
	frontendUrl *url.URL
	rpcUrl      *url.URL
	wsUrl       *url.URL
	grpcWebUrl  *url.URL
	pc          prc.Pricing
}

type Configuration struct {
	ListenUrl   string
	FrontendUrl string
	RpcUrl      string
	WsUrl       string
	GrpcWebUrl  string
}

func Run(
	ctx context.Context,
	config *Configuration,
	router rtr.Router,
) (signalC <-chan error) {
	ctxC, cancel := context.WithCancel(ctx)
	var err error
	errorC := make(chan error, 1)
	signalC = errorC
	healthC := make(chan func(*healthInternal), 10)
	var proxyUrl *url.URL
	if 0 < len(config.FrontendUrl) {
		proxyUrl, err = url.Parse(config.FrontendUrl)
		if err != nil {
			errorC <- err
			cancel()
			return
		}
	}

	handle, err := lite.Create(ctxC)
	if err != nil {
		errorC <- err
		cancel()
		return
	}
	fakeBidder, err := sgo.NewRandomPrivateKey()
	if err != nil {
		errorC <- err
		cancel()
		return
	}
	pc := prc.Create(ctxC, router, fakeBidder.PublicKey(), handle)

	var rpcUrl *url.URL
	rpcUrl, err = url.Parse(config.RpcUrl)
	if err != nil {
		errorC <- err
		cancel()
		return
	}

	var wsUrl *url.URL
	wsUrl, err = url.Parse(config.WsUrl)
	if err != nil {
		errorC <- err
		cancel()
		return
	}

	var grpcWebUrl *url.URL
	grpcWebUrl, err = url.Parse(config.GrpcWebUrl)
	if err != nil {
		errorC <- err
		cancel()
		return
	}

	closeErrorC := make(chan error, 1)

	e1 := external{
		ctx:         ctxC,
		closeErrorC: closeErrorC,
		healthC:     healthC,
		router:      router,
		frontendUrl: proxyUrl,
		rpcUrl:      rpcUrl,
		wsUrl:       wsUrl,
		grpcWebUrl:  grpcWebUrl,
		pc:          pc,
	}

	server := &http.Server{
		Addr:        config.ListenUrl,
		Handler:     e1,
		ReadTimeout: 5 * time.Second,
	}

	go loopClose(ctxC, cancel, closeErrorC, server)
	go loopServe(server, errorC)
	go loopHealth(ctxC, healthC)

	e1.has_started()

	return
}

func loopServe(server *http.Server, errorC chan<- error) {
	errorC <- server.ListenAndServe()
}

func loopClose(ctx context.Context, cancel context.CancelFunc, closeErrorC <-chan error, server *http.Server) {
	defer cancel()
	select {
	case <-ctx.Done():
	case err := <-closeErrorC:
		log.Debug(err)
	}
	server.Shutdown(context.Background())
}
