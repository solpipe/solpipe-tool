package pipeline_test

import (
	"context"
	"testing"
	"time"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state"
	sle "github.com/solpipe/solpipe-tool/test/single"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/joho/godotenv"
)

func TestController(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(func() {
		time.Sleep(3 * time.Second)
	})

	sbox, err := sle.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	relayConfig := sbox.PipelineConfig()
	err = relayConfig.Check()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("program id=%s", cba.ProgramID.String())
	t.Logf("controller admin=%s", sbox.ControllerAdmin.PublicKey().String())
	{
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
			sbox.ControllerAdmin.PublicKey(),
			100*sgo.LAMPORTS_PER_SOL,
		)
		if err != nil {
			t.Fatal(err)
		}
		mintResult, err := script.CreateMintDirect(
			sbox.Mint,
			sbox.Faucet,
			sbox.MintAuthority,
			2,
		)
		if err != nil {
			t.Fatal(err)
		}
		err = script.CreateController(
			sbox.ControllerAdmin,
			sbox.ControllerAdmin,
			*mintResult.Id,
			&state.Rate{N: 5, D: 1000},
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
	{
		data, err := router.Controller.Data()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("controller pc vault=%s ", data.PcVault)
	}

}
