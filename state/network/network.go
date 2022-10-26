package network

import (
	"context"

	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

// Calculate the network TPS so that we can calculate the face-value TPS of each validator.
type Network struct {
	ctx         context.Context
	Controller  ctr.Controller
	internalC   chan<- func(*internal)
	blockReqC   chan<- sub2.ResponseChannel[BlockTransactionCount]
	networkReqC chan<- sub2.ResponseChannel[NetworkStatus]
	voteReqC    chan<- sub2.ResponseChannel[VoteStake]
}

func Create(ctx context.Context, controller ctr.Controller, rpcClient *sgorpc.Client, wsClient *sgows.Client) (n Network, err error) {
	internalC := make(chan func(*internal), 10)

	var sub *sgows.BlockSubscription
	sub, err = wsClient.BlockSubscribe(sgows.NewBlockSubscribeFilterAll(), &sgows.BlockSubscribeOpts{
		Commitment:         sgorpc.CommitmentFinalized,
		TransactionDetails: sgorpc.TransactionDetailsNone,
	})
	if err != nil {
		return
	}
	slotSub := controller.SlotHome()
	blockCountHome := sub2.CreateSubHome[BlockTransactionCount]()
	networkStatsHome := sub2.CreateSubHome[NetworkStatus]()
	voteHome := sub2.CreateSubHome[VoteStake]()
	n = Network{
		ctx:         ctx,
		Controller:  controller,
		internalC:   internalC,
		blockReqC:   blockCountHome.ReqC,
		networkReqC: networkStatsHome.ReqC,
		voteReqC:    voteHome.ReqC,
	}

	rawC := make(chan *sgows.BlockResult, 10)
	ctxIn, cancelIn := context.WithCancel(ctx)
	go loopVoteUpdate(ctxIn, cancelIn, rpcClient, voteHome)
	go loopCountBlock(ctxIn, cancelIn, rawC, blockCountHome)
	go loopInternal(ctxIn, cancelIn, internalC, rpcClient, sub, slotSub, rawC)
	// this one is last since we need the OnBlock subscription
	go loopTps(ctxIn, cancelIn, networkStatsHome, n.OnBlock())
	return
}

func (e1 Network) OnTotalStake() sub2.Subscription[VoteStake] {
	return e1.OnVoteStake(ZeroPub())
}

func (e1 Network) OnVoteStake(vote sgo.PublicKey) sub2.Subscription[VoteStake] {
	return sub2.SubscriptionRequest(e1.voteReqC, func(x VoteStake) bool {
		if x.Id.Equals(vote) {
			return true
		} else {
			return false
		}
	})
}

func (e1 Network) OnNetworkStats() sub2.Subscription[NetworkStatus] {
	return sub2.SubscriptionRequest(e1.networkReqC, func(x NetworkStatus) bool {
		return true
	})
}

func (e1 Network) OnBlock() sub2.Subscription[BlockTransactionCount] {
	return sub2.SubscriptionRequest(e1.blockReqC, func(x BlockTransactionCount) bool { return true })
}

func (e1 Network) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}
