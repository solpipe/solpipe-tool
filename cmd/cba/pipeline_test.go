package main_test

import (
	"context"
	"errors"
	"testing"
	"time"

	cba "github.com/solpipe/cba"
	ap "github.com/solpipe/solpipe-tool/agent/pipeline"
	"github.com/solpipe/solpipe-tool/proto/test"
	"github.com/solpipe/solpipe-tool/state"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/test/sandbox"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

func TestPipeline(t *testing.T) {
	t.Parallel()
	log.Info("starting pipeline.....")
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
	//relayConfig := child.RelayConfig(admin)

	_, err = child.Client.Airdrop(ctx, &test.AirdropRequest{
		Pubkey: admin.PublicKey().String(),
		Amount: 10 * sgo.LAMPORTS_PER_SOL,
	})
	if err != nil {
		t.Fatal(err)
	}
	wallet, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	_, err = child.Client.Airdrop(ctx, &test.AirdropRequest{
		Pubkey: wallet.PublicKey().String(),
		Amount: 140 * sgo.LAMPORTS_PER_SOL,
	})
	if err != nil {
		t.Fatal(err)
	}
	relayConfig := child.RelayConfig(admin)
	args := new(ap.InitializationArg)
	args.Relay = &relayConfig
	args.Program = &ap.Configuration{
		ProgramIdCba: cba.ProgramID.ToPointer(),
		Pipeline:     nil,
		Wallet:       &wallet,
		Settings: &pipe.PipelineSettings{
			CrankFee:    &state.Rate{N: 1, D: 10},
			DecayRate:   &state.Rate{N: 3, D: 10},
			PayoutShare: &state.Rate{N: 4, D: 10},
		},
	}

	t.Logf("need to create pipeline")
	var resultC <-chan ap.ListenResult
	args.Program.Pipeline = new(sgo.PublicKey)
	resultC, *args.Program.Pipeline, err = ap.Initialize(
		ctx,
		child.Router,
		1*time.Minute,
		args,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	var agent ap.Agent
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case result := <-resultC:
		err = result.Error
		if err == nil {
			agent = result.Agent
		}
	}
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(30 * time.Second)
	agent.Close()
	// create pipeline
}
