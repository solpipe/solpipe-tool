package main_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	ak "github.com/solpipe/solpipe-tool/agent/cranker"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/test/sandbox"
)

func TestCranker(t *testing.T) {
	t.Parallel()
	log.Info("starting cranker.....")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	child, err := sandbox.Dial(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	admin, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	relayConfig := relay.CreateConfiguration(
		child.Version,
		admin,
		child.RpcUrl,
		child.WsUrl,
		http.Header{},
		"",
		nil,
	)

	all, err := sub.FetchProgramAll(ctx, child.Rpc, child.Version)
	if err != nil {
		t.Fatal(all)
	}
	controller, err := ctr.CreateController(ctx, child.Rpc, child.Ws, child.Version)
	if err != nil {
		t.Fatal(err)
	}
	network, err := ntk.Create(ctx, controller, child.Rpc, child.Ws)
	if err != nil {
		t.Fatal(err)
	}
	router, err := rtr.CreateRouter(
		ctx,
		network,
		child.Rpc,
		child.Ws,
		all,
		child.Version,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("need to create cranker")
	agent, err := ak.Create(
		ctx,
		&relayConfig,
		3*sgo.LAMPORTS_PER_SOL,
		router,
	)
	if err != nil {
		t.Fatal(err)
	}

	finishC := agent.CloseSignal()
	select {
	case <-ctx.Done():
		t.Fatal("canceled")
	case <-time.After(3 * time.Minute):
	case err = <-finishC:
		if err != nil {
			t.Fatal(err)
		}
	}
}
