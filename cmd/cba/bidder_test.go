package main_test

import (
	"context"
	"testing"

	ctr "github.com/solpipe/solpipe-tool/state/controller"
	"github.com/solpipe/solpipe-tool/test/sandbox"
	log "github.com/sirupsen/logrus"
)

func TestBidder(t *testing.T) {
	t.Parallel()
	log.Info("starting bidder.....")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	child, err := sandbox.Dial(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	controller, err := ctr.CreateController(ctx, child.Rpc, child.Ws, child.Version)
	if err != nil {
		t.Fatal(err)
	}
	data, err := controller.Data()
	if err != nil {
		t.Fatal(err)
	}
	t.Fatalf("data=%+v", data)
}
