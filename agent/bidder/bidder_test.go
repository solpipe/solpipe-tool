package bidder_test

import (
	"context"
	"net"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	bdr "github.com/solpipe/solpipe-tool/agent/bidder"
	sle "github.com/solpipe/solpipe-tool/test/single"
)

func TestBidder(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	log.SetLevel(log.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())

	t.Cleanup(func() {
		cancel()
	})
	{
		// cancel the test early by running
		// echo "" | nc 127.0.0.1 3002
		cancelCopy := cancel
		go func() {
			l, err2 := net.Listen("tcp", ":3002")
			if err2 == nil {
				c, err2 := l.Accept()
				if err2 == nil {
					c.Write([]byte("bye!"))
					cancelCopy()
				}

			}
		}()
	}

	sbox, err := sle.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	relayConfig, err := sbox.BidderConfig(0)
	if err != nil {
		t.Fatal(err)
	}

	balance, err := relayConfig.Rpc().GetBalance(
		ctx,
		relayConfig.Admin.PublicKey(),
		sgorpc.CommitmentFinalized,
	)
	if err != nil || balance.Value == 0 {
		ctxT, cancelT := context.WithTimeout(ctx, 1*time.Minute)
		defer cancelT()
		script, err := sbox.Script(ctxT)
		if err != nil {
			t.Fatal(err)
		}
		err = script.SetTx(sbox.Faucet)
		if err != nil {
			t.Fatal(err)
		}

		err = script.Transfer(
			sbox.Faucet,
			relayConfig.Admin.PublicKey(),
			10*sgo.LAMPORTS_PER_SOL,
		)
		if err != nil {
			t.Fatal(err)
		}

		err = script.FinishTx(true)
		if err != nil {
			t.Fatal(err)
		}
		balance, err := relayConfig.Rpc().GetBalance(
			ctxT,
			relayConfig.Admin.PublicKey(),
			sgorpc.CommitmentFinalized,
		)
		if err != nil {
			t.Fatal(err)
		}
		if balance.Value == 0 {
			t.Fatal("no balance")
		}
	}
	router, err := relayConfig.Router(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pcVaultId, _, err := sgo.FindAssociatedTokenAddress(
		relayConfig.Admin.PublicKey(),
		sbox.Mint.PublicKey(),
	)
	if err != nil {
		t.Fatal(err)
	}

	pcVault := new(sgotkn.Account)
	err = relayConfig.Rpc().GetAccountDataBorshInto(ctx, pcVaultId, pcVault)
	if err != nil {
		s1, err := relayConfig.ScriptBuilder(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = s1.SetTx(sbox.Faucet)
		if err != nil {
			t.Fatal(err)
		}
		err = s1.CreateTokenAccount(sbox.Faucet, relayConfig.Admin.PublicKey(), sbox.Mint.PublicKey())
		if err != nil {
			t.Fatal(err)
		}
		err = s1.FinishTx(true)
		if err != nil {
			t.Fatal(err)
		}
		err = relayConfig.Rpc().GetAccountDataBorshInto(ctx, pcVaultId, pcVault)
		if err != nil {
			t.Fatal(err)
		}
	}
	wsClient, err := relayConfig.Ws(ctx)
	if err != nil {
		t.Fatal(err)
	}

	agent, err := bdr.Create(
		ctx,
		relayConfig.Rpc(),
		wsClient,
		relayConfig.Admin,
		pcVaultId,
		pcVault,
		router,
	)
	if err != nil {
		t.Fatal(err)
	}
	closeC := agent.CloseSignal()
	t.Cleanup(func() {
		<-closeC
		log.Debug("exiting")
	})

	time.Sleep(10 * time.Minute)

}
