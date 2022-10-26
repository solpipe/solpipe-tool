package cranker_test

import (
	"context"
	"testing"
	"time"

	ckr "github.com/solpipe/solpipe-tool/agent/cranker"
	sle "github.com/solpipe/solpipe-tool/test/single"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func TestCranker(t *testing.T) {
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

	sbox, err := sle.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sbox.PipelineConfig()
	relayConfig := sbox.CrankerConfig()

	router, err := relayConfig.Router(ctx)
	if err != nil {
		t.Fatal(err)
	}

	balanceThreshold := 5 * sgo.LAMPORTS_PER_SOL
	{
		bh, err := relayConfig.Rpc().GetBalance(
			ctx,
			relayConfig.Admin.PublicKey(),
			sgorpc.CommitmentFinalized,
		)
		if err != nil {
			t.Fatal(err)
		}
		if bh.Value < 3*balanceThreshold {
			_, err = relayConfig.Rpc().RequestAirdrop(
				ctx,
				relayConfig.Admin.PublicKey(),
				3*balanceThreshold-bh.Value,
				sgorpc.CommitmentFinalized,
			)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	cranker, err := ckr.Create(
		ctx,
		&relayConfig,
		balanceThreshold,
		router,
	)
	if err != nil {
		t.Fatal(err)
	}
	finishC := cranker.CloseSignal()
	t.Cleanup(func() {
		<-finishC
	})
	select {
	case <-time.After(4 * time.Minute):
		cranker.Close()
	case err = <-cranker.CloseSignal():
	}
	if err != nil {
		t.Fatal(err)
	}
}
