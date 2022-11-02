package staker_test

import (
	"context"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	sle "github.com/solpipe/solpipe-tool/test/single"
)

func TestDelegateStake(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	log.SetLevel(log.DebugLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(func() {
		time.Sleep(3 * time.Second)
	})
	//doneC := ctx.Done()

	sbox, err := sle.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	relayConfig := sbox.StakerConfig()
	err = relayConfig.Check()
	if err != nil {
		t.Fatal(err)
	}

	solShell, err := sbox.ShellSolana()
	if err != nil {
		t.Fatal(err)
	}

	sig, err := solShell.CreateStakeDirect(
		ctx,
		sbox.Faucet,
		sbox.Stake,
		relayConfig.Admin,
		1000*sgo.LAMPORTS_PER_SOL,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sig=%s", sig.String())

	sig, err = solShell.DelegateStake(
		ctx,
		sbox.Faucet,
		sbox.Stake.PublicKey(),
		sbox.StakerAdmin,
		sbox.Vote.PublicKey(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sig=%s", sig.String())
}
