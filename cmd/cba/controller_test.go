package main_test

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/test/people"
	"github.com/solpipe/solpipe-tool/test/sandbox"
	"github.com/solpipe/solpipe-tool/util"
)

func TestController(t *testing.T) {
	t.Parallel()
	log.Info("starting controller.....")
	var err error
	//............... set everything up ..........................
	ctx, cancel := context.WithCancel(context.Background())
	go loopListenShutdown(ctx, cancel)
	wg := &sync.WaitGroup{}
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	// validatorCount int, stakerCount int, peopleCount uint64
	validatorCount, err := strconv.Atoi(os.Getenv("VALIDATOR_COUNT"))
	if err != nil {
		t.Fatal(err)
	}
	peopleCount, err := strconv.Atoi(os.Getenv("PEOPLE_COUNT"))
	if err != nil {
		t.Fatal(err)
	}
	if peopleCount < 0 {
		t.Fatal("people count is negative")
	}
	stakerCount, err := strconv.Atoi(os.Getenv("STAKER_COUNT"))
	if err != nil {
		t.Fatal(err)
	}

	ts, err := setupEverything(ctx, wg, t, stakerCount, uint64(peopleCount))
	if err != nil {
		t.Fatal(err)
	}
	sb := ts.Sandbox
	err = sb.AddValidator(ctx, wg, validatorCount)
	if err != nil {
		t.Fatal(err)
	}
	scriptConfig := script.Configuration{Version: vrs.VERSION_1}
	err = sb.MintDb.Default(ctx, *sb.Faucet, scriptConfig, ts.Rpc, ts.Ws)
	if err != nil {
		t.Fatal(err)
	}

	version := scriptConfig.Version
	s1, err := script.Create(ctx, &script.Configuration{
		Version: version,
	}, ts.Rpc, ts.Ws)
	if err != nil {
		t.Fatal(err)
	}
	{
		payer := *sb.Faucet
		err = s1.SetTx(payer)
		if err != nil {
			t.Fatal(err)
		}

		admin := sb.People.Users[0]
		reBal, err := ts.Rpc.GetMinimumBalanceForRentExemption(ctx, util.STRUCT_SIZE_CONTROLLER+util.STRUCT_SIZE_MINT+util.STRUCT_SIZE_PERIOD_RING+util.STRUCT_SIZE_BID_LIST, sgorpc.CommitmentFinalized)
		if err != nil {
			t.Fatal(err)
		}
		err = s1.Transfer(*sb.Faucet, admin.Account(), reBal+10)
		if err != nil {
			t.Fatal(err)
		}
		mint, present := sb.MintDb.Mints[sandbox.MINT_USD]
		if !present {
			t.Fatal("missing mint")
		}

		err = s1.CreateController(admin.Key, admin.Key, *mint.Id, &state.Rate{
			N: 5, D: 1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		err = s1.FinishTx(true)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("admin=%s", admin.Key.PublicKey().String())
	}
	var signalC <-chan error
	signalC, err = ts.CreateGrpc(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	shutdownC := make(chan error, 1)
	go func() {
		l, err2 := net.Listen("tcp", ":3001")
		if err2 == nil {
			c, err2 := l.Accept()
			if err2 == nil {
				c.Write([]byte("bye!\n"))
				c.Close()
			}
			l.Close()
			<-shutdownC
		}
	}()

	err = ts.Sandbox.People.Prepare(&people.Preparation{
		Ctx:    ctx,
		Mint:   sb.MintDb.List(),
		Faucet: *ts.Sandbox.Faucet,
		// TODO: get value from Sandbox object
		RpcUrl:     "http://localhost:8899",
		WsUrl:      "ws://localhost:8900",
		SeedAmount: 1 * sgo.LAMPORTS_PER_SOL,
	})
	if err != nil {
		return
	}

	t.Log("waiting for tests to finish")
	select {
	case <-ctx.Done():
		log.Debug("canceled")
	case <-shutdownC:
		log.Debug("shutdown trigger")
	case <-signalC:
		log.Debug("heart beat signal")
	case <-time.After(30 * time.Minute):
		log.Debug("time out")
	}

	t.Log("controller has finished")
}

func setupEverything(ctx context.Context, wg *sync.WaitGroup, t *testing.T, stakerCount int, peopleCount uint64) (ts *sandbox.TestSetup, err error) {

	// we need a keypair and bpf file to deploy to the local validator
	programIdKeyPair, present := os.LookupEnv("PROGRAM_ID_CBA_KEYPAIR")
	if !present {
		err = errors.New("no cba program key pair")
		return
	}
	programBpf, present := os.LookupEnv("PROGRAM_ID_CBA_BPF")
	if !present {
		err = errors.New("no cba program bpf")
		return
	}

	log.Infof("Program ID=%s", cba.ProgramID.String())

	ledgerDir := ""

	ts, err = sandbox.RunTestValidator(ctx, wg, ledgerDir, stakerCount, peopleCount)
	if err != nil {
		return
	}
	t.Cleanup(func() {
		os.RemoveAll(ts.Sandbox.WorkingDir())
	})
	sb := ts.Sandbox
	err = sb.Deploy(ctx, programIdKeyPair, programBpf)
	if err != nil {
		return
	}

	return
}

// shutdown the test by echo "" | nc 127.0.0.1 3001
func loopListenShutdown(ctx context.Context, cancel context.CancelFunc) {

	defer cancel()
	l, err := net.Listen("tcp", ":3001")
	if err != nil {
		log.Error(err)
		return
	}
	go loopListenShutdownContext(ctx, l)
	c, err := l.Accept()
	if err != nil {
		//log.Error(err)
		return
	}
	c.Write([]byte("bye!\n"))
	c.Close()
	l.Close()
	cancel()
}

func loopListenShutdownContext(ctx context.Context, l net.Listener) {
	<-ctx.Done()
	l.Close()
}

/*
import (
	"context"
	"testing"
	"time"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)


func TestController(t *testing.T) {
	var err error
	err = godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	err = godotenv.Load("../../.tmpenv")
	if err != nil {
		t.Fatal(err)
	}
	log.SetLevel(log.DebugLevel)

	err = util.SetProgramID()
	if err != nil {
		t.Fatal(err)
	}
	log.Debugf("program id=%s", cba.ProgramID.String())

	ctx, cancel := context.WithCancel(context.Background())
	//wg := &sync.WaitGroup{}
	t.Cleanup(func() {
		cancel()
		//wg.Wait()
	})

	rpcClient, wsClient, err := util.RpcConnect(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	payer, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	err = script.Airdrop(ctx, rpcClient, wsClient, payer.PublicKey(), 15*sgo.LAMPORTS_PER_SOL)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	err = script.Airdrop(ctx, rpcClient, wsClient, admin.PublicKey(), 15*sgo.LAMPORTS_PER_SOL)
	if err != nil {
		t.Fatal(err)
	}

	// create mint
	usdcMintAuthority, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	usdcMintResult, err := script.CreateMint(ctx, rpcClient, wsClient, payer, usdcMintAuthority, 2)
	if err != nil {
		t.Fatal(err)
	}
	mint := *usdcMintResult.Id

	version := vrs.VERSION_1

	s1, err := script.Create(ctx, &script.Configuration{
		Version: version,
	}, rpcClient, wsClient)
	if err != nil {
		t.Fatal(err)
	}
	err = s1.SetTx(payer)
	if err != nil {
		t.Fatal(err)
	}

	err = s1.CreateController(admin, admin, mint, &state.Rate{
		N: 5, D: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s1.FinishTx(true)
	if err != nil {
		t.Fatal(err)
	}

	controller, err := ctr.CreateController(ctx, rpcClient, wsClient, version)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("controller id is %s", controller.Id().String())
	time.Sleep(30 * time.Second)
	//wg.Add(1)
	go func() {
		<-controller.CloseSignal()
		//wg.Done()
	}()

}
*/
