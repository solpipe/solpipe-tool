package web

import (
	"context"

	"net/http"
	"net/url"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/ds/ts/lite"
	prc "github.com/solpipe/solpipe-tool/state/pricing"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	ctx         context.Context
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
	var err error
	errorC := make(chan error, 1)
	signalC = errorC
	healthC := make(chan func(*healthInternal), 10)
	var proxyUrl *url.URL
	if 0 < len(config.FrontendUrl) {
		proxyUrl, err = url.Parse(config.FrontendUrl)
		if err != nil {
			errorC <- err
			return
		}
	}

	handle, err := lite.Create(ctx)
	if err != nil {
		errorC <- err
		return
	}
	fakeBidder, err := sgo.NewRandomPrivateKey()
	if err != nil {
		errorC <- err
		return
	}
	pc := prc.Create(ctx, router, fakeBidder.PublicKey(), handle)

	var rpcUrl *url.URL
	rpcUrl, err = url.Parse(config.RpcUrl)
	if err != nil {
		errorC <- err
		return
	}

	var wsUrl *url.URL
	wsUrl, err = url.Parse(config.WsUrl)
	if err != nil {
		errorC <- err
		return
	}

	var grpcWebUrl *url.URL
	grpcWebUrl, err = url.Parse(config.GrpcWebUrl)
	if err != nil {
		errorC <- err
		return
	}

	e1 := external{
		ctx:         ctx,
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
	go loopClose(ctx, server)
	go loopServe(server, errorC)
	go loopHealth(ctx, healthC)

	e1.has_started()

	return
}

func loopServe(server *http.Server, errorC chan<- error) {
	errorC <- server.ListenAndServe()
}

func loopClose(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	server.Shutdown(context.Background())
}
