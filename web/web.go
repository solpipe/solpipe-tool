package web

import (
	"context"

	"net/http"
	"net/url"
	"time"

	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	ctx      context.Context
	healthC  chan func(*healthInternal)
	router   rtr.Router
	proxyUrl *url.URL
}

type Configuration struct {
	ListenUrl   string
	FrontendUrl string
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
	e1 := external{
		ctx:      ctx,
		healthC:  healthC,
		router:   router,
		proxyUrl: proxyUrl,
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
