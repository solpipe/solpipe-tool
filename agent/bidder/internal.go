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
	bin "github.com/gagliardetto/binary"
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
	pcVaultId                sgo.PublicKey
	pcVault                  *sgotkn.Account // the token account storing funds used to bid for bandwidth
	pcVaultSub               *sgows.AccountSubscription
	router                   rtr.Router
	connC                    chan<- pipelineConnectionStatus
	periodRingC              chan<- periodUpdate
	bidListC                 chan<- bidUpdate
	brain                    *brainInfo
	timeHome                 *timeHome
	slot                     uint64
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
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	bidder sgo.PrivateKey,
	pcVaultId sgo.PublicKey,
	pcVault *sgotkn.Account,
	router rtr.Router,
	bsg *brainSubGroup,
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
	in.pcVaultId = pcVaultId
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
	in.slot = 0

	// do brain preparation here
	in.brain = createBrainInfo()
	defer bsg.Close()

	pipelineSub := router.ObjectOnPipeline()
	defer pipelineSub.Unsubscribe()

	netstatSub := router.Network.OnNetworkStats()
	defer netstatSub.Unsubscribe()

	slotSub := router.Controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()

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
		case id := <-bsg.budget.DeleteC:
			bsg.budget.Delete(id)
		case req := <-bsg.budget.ReqC:
			bsg.budget.Receive(req)
		case id := <-bsg.netstats.DeleteC:
			bsg.budget.Delete(id)
		case req := <-bsg.netstats.ReqC:
			bsg.netstats.Receive(req)
		case err = <-in.pcVaultSub.RecvErr():
			break out
		case d := <-in.pcVaultSub.RecvStream():
			x, ok := d.(*sgows.AccountResult)
			if !ok {
				err = errors.New("bad account result")
				break out
			}
			in.pcVault = new(sgotkn.Account)
			err = bin.UnmarshalBorsh(in.pcVault, x.Value.Data.GetBinary())
			if err != nil {
				break out
			}
			log.Debug("bidder balance in pc vault is %d", in.pcVault.Amount)
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
				in.timeline_on_bid(y, x.list)
			}
		case x := <-periodRingUpdateC:
			y, present := in.pipelines[x.id.String()]
			if present {
				in.timeline_on_period(y, x.ring)
			}
		case x := <-pipelineConnectionStatusC:
			in.on_connection_status(x)
		case err = <-slotSub.ErrorC:
			break out
		case in.slot = <-slotSub.StreamC:
			in.on_slot()
		}
	}
	in.finish(err)
}

func (in *internal) init() error {
	var err error
	in.init_timeline()

	in.tor, err = proxy.SetupTor(in.ctx, true)
	if err != nil {
		return err
	}
	list, err := in.router.AllPipeline()
	if err != nil {
		return err
	}
	in.pcVaultSub, err = in.ws.AccountSubscribe(in.pcVaultId, sgorpc.CommitmentFinalized)
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
	in.pcVaultSub.Unsubscribe()
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
