package pipeline_test

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ap "github.com/solpipe/solpipe-tool/agent/pipeline"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/state"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	sle "github.com/solpipe/solpipe-tool/test/single"
)

func TestSetup(t *testing.T) {
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

	{
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
	relayConfig := sbox.PipelineConfig()
	err = relayConfig.Check()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pipeline admin=%s", sbox.PipelineAdmin.PublicKey().String())
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
	torMgr, err := proxy.SetupTor(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	var agent ap.Agent
	pipeline, err := router.PipelineById(sbox.Pipeline.PublicKey())
	if err != nil {
		wallet := sbox.Faucet
		args := new(ap.InitializationArg)
		args.Relay = &relayConfig
		args.Program = &ap.Configuration{
			ProgramIdCba: cba.ProgramID.ToPointer(),
			Pipeline:     nil,
			Wallet:       &wallet,
			Settings: &pipe.PipelineSettings{
				CrankFee:    &state.Rate{N: 1, D: 10},
				PayoutShare: &state.Rate{N: 4, D: 10},
				BidSpace:    100,
				RefundSpace: 20,
			},
		}
		var resultC <-chan ap.ListenResult
		args.Program.Pipeline = new(sgo.PublicKey)
		pid := sbox.Pipeline
		resultC, *args.Program.Pipeline, err = ap.Initialize(
			ctx,
			router,
			1*time.Minute,
			args,
			&pid,
			args.Program.Settings.BidSpace,
			args.Program.Settings.RefundSpace,
			torMgr,
		)
		if err != nil {
			t.Fatal(err)
		}

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
	} else {
		data, err := pipeline.Data()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("pipeline admin=%s", data.Admin.String())
		ring, err := pipeline.PeriodRing()
		if err != nil {
			t.Fatal(err)
		}
		t.Log("ring....")
		for i := uint16(0); i < ring.Length; i++ {
			k := (ring.Start + i) % uint16(len(ring.Ring))
			t.Logf("ring[%d]=%+v", i, ring.Ring[k])
		}

		pipelineConfig := sbox.PipelineConfig()
		configFp := os.Getenv("PIPELINE_CONFIG")
		log.Debugf("period appending config=%s", configFp)
		agent, err = ap.Create(
			ctx,
			&ap.InitializationArg{
				Relay: &pipelineConfig,
				Program: &ap.Configuration{
					ProgramIdCba: cba.ProgramID.ToPointer(),
					Pipeline:     pipeline.Id.ToPointer(),
					Wallet:       &pipelineConfig.Admin,
					Settings: &pipe.PipelineSettings{
						CrankFee:    &state.Rate{N: 1, D: 100},
						PayoutShare: &state.Rate{N: 95, D: 100},
						BidSpace:    100,
						RefundSpace: 20,
					},
				},
				ConfigFilePath: configFp,
			},
			router,
			pipeline,
			torMgr,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		<-agent.CloseSignal()
		t.Log("agent closed")
	})
	{
		timeC := time.After(30 * time.Minute)
		newPeriods := 0
		sub := pipeline.OnPayout()
		defer sub.Unsubscribe()
		killedC := agent.CloseSignal()
	out1:
		for newPeriods < 3 {
			select {
			case <-killedC:
				err = errors.New("agent died")
			case <-timeC:
				err = errors.New("timed out")
			case <-ctx.Done():
				err = errors.New("canceled")
			case err = <-sub.ErrorC:
			case x := <-sub.StreamC:
				log.Infof("new period %d -> %d", x.Data.Period.Start, x.Data.Period.Start+x.Data.Period.Length)
				newPeriods++
			}
			if err != nil {
				log.Debug(err)
				break out1
			}
		}
		if newPeriods < 3 {
			t.Fatal("failed to add periods")
		}
	}

	select {
	case <-ctx.Done():
	case <-time.After(20 * time.Minute):
		agent.Close()
	}

}
