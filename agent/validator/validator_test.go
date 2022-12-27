package validator_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	av "github.com/solpipe/solpipe-tool/agent/validator"
	valadmin "github.com/solpipe/solpipe-tool/agent/validator/admin"
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
	doneC := ctx.Done()

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
	relayConfig := sbox.ValidatorConfig()
	err = relayConfig.Check()
	if err != nil {
		t.Fatal(err)
	}

	router, err := relayConfig.Router(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var agent av.Agent
	adminSocket := "unix:///tmp/validator.socket"
	validator, err := router.ValidatorByVote(sbox.Vote.PublicKey())
	if err != nil {
		args := new(av.InitializationArg)
		args.Wallet = sbox.Faucet
		args.AdminListenUrl = adminSocket
		args.ControllerId = router.Controller.Id()
		args.Stake = sbox.Stake.PublicKey()
		args.Vote = sbox.Vote
		args.RelayConfig = relayConfig

		resultGroup, err := av.Initialize(
			ctx,
			router,
			10*time.Minute,
			args,
		)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-doneC:
			t.Fatal("canceled")
		case err = <-resultGroup.ErrorC:
			t.Fatal(err)
		case agent = <-resultGroup.AgentC:
		}
	} else {
		data, err := validator.Data()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("validator admin=%s", data.Admin.String())
		agent, err = av.Create(
			ctx,
			relayConfig,
			router,
			validator,
			"/tmp/validator-config.json", // TODO: change this
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	{
		closeC := agent.CloseSignal()
		t.Cleanup(func() {
			<-closeC
			t.Log("agent closed")
		})
	}

	pipeline, err := router.PipelineById(sbox.Pipeline.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	switchedC := make(chan error, 1)
	go loopPipeline(ctx, pipeline, switchedC, validator.Id)

	admin, err := valadmin.Dial(ctx, adminSocket)
	if err != nil {
		t.Fatal(err)
	}
	err = admin.SetPipeline(ctx, sbox.Pipeline.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(120 * time.Minute):
	case err = <-switchedC:
	}

	if err != nil {
		t.Fatal(err)
	}
}

func loopPipeline(
	ctx context.Context,
	pipeline pipe.Pipeline,
	switchedC chan<- error,
	validatorId sgo.PublicKey,
) {
	sub := pipeline.OnValidator()
	defer sub.Unsubscribe()
	doneC := ctx.Done()
	var err error
	select {
	case <-doneC:
		err = nil
	case err = <-sub.ErrorC:
	case x := <-sub.StreamC:
		if validatorId.Equals(x.Validator.Id) {
			err = nil
		} else {
			err = errors.New("wrong validator id")
		}
	}
	switchedC <- err

}
