package pipeline

import (
	"context"
	"errors"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/util"
)

const PIPELINE_MEMBER_SIZE = 1500000

// Create a pipeline account on chain and run an agent.
func Initialize(
	ctx context.Context,
	router rtr.Router,
	timeout time.Duration,
	args *InitializationArg,
	pipelineIdKeypair *sgo.PrivateKey,
	bidSpace uint16,
	residualSpace uint16,
	torMgr *tor.Tor,
) (
	resultC <-chan ListenResult,
	pipelineId sgo.PublicKey,
	err error,
) {
	controller := router.Controller
	if args == nil {
		err = errors.New("no config")
		return
	}
	err = args.Check()
	if err != nil {
		return
	}
	rpcClient := args.Relay.Rpc()
	wsClient, err := args.Relay.Ws(ctx)
	if err != nil {
		return
	}
	programConfig := args.Program
	// TODO: fix this; use a precise number
	minSize := util.STRUCT_SIZE_PIPELINE + util.STRUCT_SIZE_BID_LIST + util.STRUCT_SIZE_PERIOD_RING + 2449920

	minRent, err := rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		minSize,
		sgorpc.CommitmentFinalized,
	)
	if err != nil {
		return
	}

	var adminBalance uint64
	createAdminAccount := false
	{
		var x *sgorpc.GetBalanceResult
		x, err = rpcClient.GetBalance(ctx, args.Admin().PublicKey(), sgorpc.CommitmentFinalized)
		if err != nil {
			return
		}
		if x.Value == 0 {
			_, err = rpcClient.GetAccountInfo(ctx, args.Admin().PublicKey())
			if err != nil {
				err = nil
				createAdminAccount = true
			}
		}
		adminBalance = x.Value
	}

	s1, err := script.Create(
		ctx,
		&script.Configuration{Version: args.Relay.Version},
		rpcClient,
		wsClient,
	)
	if err != nil {
		return
	}
	err = s1.SetTx(*programConfig.Wallet)
	if err != nil {
		return
	}
	if createAdminAccount {
		_, err = s1.CreateAccount(0, args.Admin().PublicKey(), *programConfig.Wallet)
		if err != nil {
			return
		}
	}
	if adminBalance < minRent {
		err = s1.Transfer(*programConfig.Wallet, args.Admin().PublicKey(), minRent-adminBalance)
		if err != nil {
			return
		}
	}
	var pid sgo.PrivateKey
	if pipelineIdKeypair == nil {
		pid, err = s1.AddPipeline(
			controller,
			*programConfig.Wallet,
			args.Admin(),
			*programConfig.Settings.CrankFee,
			100,
			*programConfig.Settings.PayoutShare,
			script.TICKSIZE_DEFAULT,
			args.Program.Settings.RefundSpace,
		)
	} else {
		pid, err = s1.AddPipelineDirect(
			*pipelineIdKeypair,
			controller,
			*programConfig.Wallet,
			args.Admin(),
			*programConfig.Settings.CrankFee,
			100,
			*programConfig.Settings.PayoutShare,
			script.TICKSIZE_DEFAULT,
			args.Program.Settings.RefundSpace,
		)
	}

	if err != nil {
		return
	}
	pipelineId = pid.PublicKey()
	*args.Program.Pipeline = pipelineId
	resultC = CreateFromListener(ctx, args, router, 2*time.Minute, torMgr)
	err = s1.FinishTx(false)
	if err != nil {
		return
	}
	return
}

type ListenResult struct {
	Agent Agent
	Error error
}

// Call this from listener.  Return an agent once the pipeline account has been created by Initialize().
func CreateFromListener(
	ctx context.Context,
	args *InitializationArg,
	router rtr.Router,
	timeout time.Duration,
	torMgr *tor.Tor,
) <-chan ListenResult {
	resultC := make(chan ListenResult, 1)
	if args == nil {
		resultC <- ListenResult{Error: errors.New("no args")}
		return resultC
	}
	if err := args.Check(); err != nil {
		resultC <- ListenResult{Error: err}
		return resultC
	}

	go loopListener(ctx, timeout, args, router, resultC, torMgr)
	return resultC
}

func loopListener(
	ctx context.Context,
	timeout time.Duration,
	args *InitializationArg,
	router rtr.Router,
	resultC chan<- ListenResult,
	torMgr *tor.Tor,
) {

	var err error
	sh := router.ObjectOnPipeline()
	defer sh.Unsubscribe()

	var p pipe.Pipeline
out:
	for {
		select {
		case <-time.After(timeout):
			err = errors.New("canceled")
			break out
		case err = <-sh.ErrorC:
			break out
		case p = <-sh.StreamC:
			log.Debugf("listener pipeline=%s vs %s", p.Id.String(), args.Program.Pipeline.String())
			if p.Id.Equals(*args.Program.Pipeline) {
				break out
			}
		}
	}
	if err != nil {
		resultC <- ListenResult{Error: err}
		return
	}
	a, err := Create(ctx, args, router, p, torMgr)
	if err != nil {
		resultC <- ListenResult{Error: err}
		return
	}
	resultC <- ListenResult{Error: nil, Agent: a}
}
