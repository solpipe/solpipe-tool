package main_test

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	agentVal "github.com/solpipe/solpipe-tool/agent/validator"
	valadmin "github.com/solpipe/solpipe-tool/agent/validator/admin"
	pbt "github.com/solpipe/solpipe-tool/proto/test"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/test/sandbox"
	"github.com/solpipe/solpipe-tool/util"
)

func TestValidator(t *testing.T) {
	t.Parallel()
	gid := util.GetGID()
	log.Info("starting validator.....")
	child, err := sandbox.Dial(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := child.Ctx
	cancel := child.Cancel
	t.Cleanup(func() {
		cancel()
		log.Debug("validator canceled")
	})

	childValidator, err := child.PopValidator()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("(%d) validator admin=%s", gid, childValidator.Admin.String())

	router := child.Router

	log.Debug("creating wallet")
	wallet, err := createWallet(ctx, child, 1000*sgo.LAMPORTS_PER_SOL)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("(%d) wallet created", gid)
	t.Logf("(%d) creating stake", gid)
	stake, err := child.CreateStake(
		wallet,
		childValidator.Admin,
		10*sgo.LAMPORTS_PER_SOL,
	)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Minute)
	t.Logf("(%d) p - %d", gid, 1)

	log.Debugf("delegating stake=%s to vote=%s", stake.PublicKey().String(), childValidator.Vote.PublicKey().String())
	err = child.DelegateStake(
		wallet,
		childValidator.Admin,
		stake.PublicKey(),
		childValidator.Vote.PublicKey(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sleeping")
	time.Sleep(1 * time.Minute)
	t.Logf("slept")

	relayConfig := relay.CreateConfiguration(
		child.Version,
		childValidator.Admin,
		child.RpcUrl,
		child.WsUrl,
		child.Headers.Clone(),
		child.AdminListenUrl,
		&relay.ClearNetListenConfig{
			Port: 50059,
			Ipv4: net.IPv4(127, 0, 0, 1),
			Ipv6: nil,
		},
	)
	torMgr, err := proxy.SetupTor(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var agentListener agentVal.ListenResult
	agentListener, err = agentVal.Initialize(
		ctx,
		router,
		5*time.Minute,
		&agentVal.InitializationArg{
			RelayConfig:    relayConfig,
			Wallet:         wallet,
			ControllerId:   child.Controller().Id(),
			Vote:           childValidator.Vote,
			Stake:          stake.PublicKey(),
			AdminListenUrl: child.AdminListenUrl,
		},
		torMgr,
	)
	if err != nil {
		t.Fatal(err)
	}
	var agent agentVal.Agent
	err = <-agentListener.ErrorC

	if err != nil {
		t.Fatal(err)
	} else {
		agent = <-agentListener.AgentC
		agentDoneC := agent.CloseSignal()
		t.Cleanup(func() {
			log.Debug("waiting for agent to close")
			<-agentDoneC
		})

		data, err := agent.Validator.Data()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("admin %s vs %s", data.Admin.String(), childValidator.Admin.PublicKey().String())
		log.Debugf("validator data=%s", data.Vote)
		t.Logf("validator data=%s", data.Vote)
		if !data.Vote.Equals(childValidator.Vote.PublicKey()) {
			t.Fatalf("vote accounts not equal (%s vs %s)", data.Vote.String(), childValidator.Vote.PublicKey().String())
		}
		if !data.Controller.Equals(child.Controller().Id()) {
			t.Fatal("controller id does not match")
		}

		if !data.Admin.Equals(childValidator.Admin.PublicKey()) {
			t.Fatal("admin does not match")
		}
		log.Debug("validator done - 1")
	}
	log.Debug("validator done - 2")

	p, err := pickPipeline(router)
	if err != nil {
		t.Fatal(err)
	}

	adminClient, err := valadmin.Dial(ctx, child.AdminListenUrl)
	if err != nil {
		t.Fatal(err)
	}
	{
		ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
		err = adminClient.SetPipeline(ctx2, p.Id)
		if err != nil {
			cancel2()
			t.Fatal(err)
		}
		cancel2()
	}

	log.Debugf("adding validator=%s to pipeline=%s", agent.Validator.Id.String(), p.Id)
	err = dialPipeline(ctx, p, torMgr)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: connect to the agent via grpc?

	//agent.CloseSignal()

	// clean up does not happen until all parallel tests are complete
	// so call cancel now before finishing this function
	cancel()
}

func dialPipeline(ctx context.Context, pipeline pipe.Pipeline, torMgr *tor.Tor) (err error) {

	err = errors.New("not implemented yet")
	return
}

// by this time, there should be several pipelines available
func pickPipeline(router rtr.Router) (pipe.Pipeline, error) {
	pList, err := router.AllPipeline()
	if err != nil {
		return pipe.Pipeline{}, err
	}
	log.Debugf("pipeline=%d", len(pList))
	return pList[rand.Intn(len(pList))], nil
}

func createWallet(ctx context.Context, child *sandbox.TestChild, amount uint64) (wallet sgo.PrivateKey, err error) {
	wallet, err = sgo.NewRandomPrivateKey()
	if err != nil {
		return
	}
	_, err = child.Client.Airdrop(ctx, &pbt.AirdropRequest{
		Pubkey: wallet.PublicKey().String(),
		Amount: amount,
	})
	if err != nil {
		return
	}

	return
}
