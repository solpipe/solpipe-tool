package bidder

import (
	"context"
	"errors"
	"math/big"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	ct "github.com/solpipe/solpipe-tool/client"
	"github.com/solpipe/solpipe-tool/proxy"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/sub"
	"google.golang.org/grpc"
)

type internal struct {
	ctx                      context.Context
	errorC                   chan<- error
	closeSignalCList         []chan<- error
	updateChangeActiveStakeC chan<- sub.StakeUpdate
	activeStake              *big.Int
	totalStake               *big.Int
	allotedTps               *big.Float
	networkStatus            *ntk.NetworkStatus
	config                   *Configuration
	rpc                      *sgorpc.Client
	ws                       *sgows.Client
	grpcClient               *ct.Client
	controller               ctr.Controller
	tor                      *tor.Tor
	updatePipelineC          chan<- sub.PipelineGroup
	pipelines                map[string]*pipelineInfo // mape pipeline id to Pipeline
	proxyConn                map[string]*ct.Client
	updateConnC              chan<- pipelineProxyConnection
	bidder                   sgo.PrivateKey
	pcVault                  *sgotkn.Account // the token account storing funds used to bid for bandwidth
	router                   rtr.Router
	connC                    chan<- pipelineConnectionStatus
	periodRingC              chan<- periodUpdate
	bidListC                 chan<- bidUpdate
}

type pipelineProxyConnection struct {
	id   sgo.PublicKey
	conn *grpc.ClientConn
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	startErrorC chan<- error,
	configReadErrorC <-chan error,
	configC <-chan Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	bidder sgo.PrivateKey,
	pcVault *sgotkn.Account,
	router rtr.Router,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	updatePipelineC := make(chan sub.PipelineGroup, 1)
	updateConnC := make(chan pipelineProxyConnection, 10)
	updateChangeActiveStakeC := make(chan sub.StakeUpdate, 1)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.updateChangeActiveStakeC = updateChangeActiveStakeC
	in.closeSignalCList = make([]chan<- error, 0)
	in.rpc = rpcClient
	in.ws = wsClient
	in.controller = router.Controller
	in.pipelines = make(map[string]*pipelineInfo)
	in.bidder = bidder
	in.pcVault = pcVault
	in.activeStake = big.NewInt(0)
	in.totalStake = big.NewInt(1)
	in.networkStatus = nil
	in.updatePipelineC = updatePipelineC
	in.updateConnC = updateConnC
	in.proxyConn = make(map[string]*ct.Client)
	in.router = router
	pipelineConnectionStatusC := make(chan pipelineConnectionStatus, 1)
	in.connC = pipelineConnectionStatusC
	periodRingUpdateC := make(chan periodUpdate, 1)
	in.periodRingC = periodRingUpdateC
	bidListUpdateC := make(chan bidUpdate, 1)
	in.bidListC = bidListUpdateC

	pipelineSub := router.ObjectOnPipeline()
	defer pipelineSub.Unsubscribe()

	netstatSub := router.Network.OnNetworkStats()
	defer netstatSub.Unsubscribe()

	err = in.init()
	startErrorC <- err
	if err != nil {
		return
	}
	var present bool
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-netstatSub.ErrorC:
			break out
		case x := <-netstatSub.StreamC:
			in.networkStatus = &x
			in.on_relative_stake()
		case update := <-updateChangeActiveStakeC:
			in.activeStake.Add(in.activeStake, update.ActivatedStake)
			if in.activeStake.Sign() < 0 {
				err = errors.New("activate stake is negative")
				break out
			}
			in.totalStake.Set(update.TotalStake)
			in.on_relative_stake()
		case req := <-internalC:
			req(in)
		case err = <-pipelineSub.ErrorC:
			break out
		case p := <-pipelineSub.StreamC:
			_, present = in.pipelines[p.Id.String()]
			if !present {
				in.on_pipeline(p)
			}
		case x := <-bidListUpdateC:
			y, present := in.pipelines[x.id.String()]
			if present {
				y.on_bid(x.list)
			}
		case x := <-periodRingUpdateC:
			y, present := in.pipelines[x.id.String()]
			if present {
				y.on_period(x.ring)
			}
		case x := <-pipelineConnectionStatusC:
			in.on_connection_status(x)
		}
	}
	in.finish(err)
}

func (in *internal) init() error {
	var err error
	in.tor, err = proxy.SetupTor(in.ctx, true)
	if err != nil {
		return err
	}
	list, err := in.router.AllPipeline()
	if err != nil {
		return err
	}
	var present bool
	for _, p := range list {
		_, present = in.pipelines[p.Id.String()]
		if !present {
			in.on_pipeline(p)
		}
	}
	return nil
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
