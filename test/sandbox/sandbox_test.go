package sandbox_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/solpipe/solpipe-tool/script"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/test/sandbox"
)

func TestBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	ledgerDir := ""

	ts, err := sandbox.RunTestValidator(ctx, wg, ledgerDir, 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(ts.Sandbox.WorkingDir())
	})
	start := time.Now().Unix()

	//time.Sleep(60 * time.Second)
	sb := ts.Sandbox
	scriptConfig := script.Configuration{Version: vrs.VERSION_1}

	err = sb.MintDb.Default(ctx, *sb.Faucet, scriptConfig, ts.Rpc, ts.Ws)
	if err != nil {
		t.Fatal(err)
	}
	for ticker, x := range sb.MintDb.Mints {
		t.Logf("...ticker=%s; supply=%d, id=%s", ticker, x.Data.Decimals, x.Id.String())
	}
	t.Logf("diff=%d", time.Now().Unix()-start)
	t.Fatalf("faucet=%s at %s", ts.Sandbox.Faucet.PublicKey().String(), ts.Sandbox.WorkingDir())
}
