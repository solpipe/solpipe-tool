package main

import (
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/web"
)

type Web struct {
	Port        uint16 `arg name:"port" help:"the port number to listen on"`
	FrontendUrl string `option name:"frontend" help:"redirect to a front end web page"`
}

func (r *Web) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	unnecessaryAdmin, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}
	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		unnecessaryAdmin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		DEFAULT_VALIDATOR_ADMIN_SOCKET, // this line is irrelevant
		nil,
	)
	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}
	signalC := web.Run(
		ctx,
		&web.Configuration{
			ListenUrl:   fmt.Sprintf("0.0.0.0:%d", r.Port),
			FrontendUrl: r.FrontendUrl,
		},
		router,
	)
	select {
	case <-ctx.Done():
	case err = <-signalC:
	}
	if err != nil {
		return err
	}
	return nil
}
