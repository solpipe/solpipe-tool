package pricing

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx                           context.Context
	bidder                        sgo.PublicKey
	errorC                        chan<- error
	closeSignalCList              []chan<- error
	router                        rtr.Router
	pipelineM                     map[string]*pipelineInfo
	pipelineMixM                  map[string]float64 // pipeline id->% of total required capacity
	capacityRequirement           []CapacityPoint
	capacityRequirement_i         int // reference capacityRequirement array
	capacityRequirementDelayReadC chan int
	stakeC                        chan<- stakeUpdate
	pipelineDataC                 chan<- sub.PipelineGroup
	periodC                       chan<- periodUpdate
	periodM                       map[string]*ll.Node[*periodInfo]
	bidC                          chan<- bidUpdate
	ns                            ntk.NetworkStatus
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	router rtr.Router,
	bidder sgo.PublicKey,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 10)
	stakeC := make(chan stakeUpdate)
	pipelineDataC := make(chan sub.PipelineGroup)
	periodC := make(chan periodUpdate)
	bidC := make(chan bidUpdate)
	capacityRequirementDelayReadC := make(chan int)

	in := new(internal)
	in.ctx = ctx
	in.bidder = bidder
	in.errorC = errorC
	in.stakeC = stakeC
	in.pipelineDataC = pipelineDataC
	in.periodC = periodC
	in.bidC = bidC
	in.capacityRequirementDelayReadC = capacityRequirementDelayReadC
	in.closeSignalCList = make([]chan<- error, 0)
	in.router = router
	in.pipelineM = make(map[string]*pipelineInfo)
	in.periodM = make(map[string]*ll.Node[*periodInfo])
	in.ns = ntk.NetworkStatus{
		WindowSize:                       0,
		AverageTransactionsPerBlock:      0,
		AverageTransactionsPerSecond:     0,
		AverageTransactionsSizePerSecond: 0,
	}
	in.pipelineMixM = make(map[string]float64)
	in.capacityRequirement = []CapacityPoint{}

	var relStake stakeUpdate

	pipelineSub := in.router.ObjectOnPipeline()
	defer pipelineSub.Unsubscribe()

	networkSub := in.router.Network.OnNetworkStats()
	defer networkSub.Unsubscribe()

out:
	for {
		select {
		case err = <-errorC:
			break out
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		case err = <-networkSub.ErrorC:
			break out
		case in.ns = <-networkSub.StreamC:
		case relStake = <-stakeC:
			in.on_stake(relStake)
		case err = <-pipelineSub.ErrorC:
			break out
		case p := <-pipelineSub.StreamC:
			in.on_pipeline(p)
		case s := <-pipelineDataC:
			pi, p := in.pipelineM[s.Id.String()]
			if p {
				if s.IsOpen {
					pi.data = s.Data
				} else {
					// TODO: do we want to delete pipelines?
					pi.cancel()
					delete(in.pipelineM, s.Id.String())
				}
			}
		case update := <-periodC:
			in.on_period(update)
		case update := <-bidC:
			in.on_bid(update)
		case i := <-capacityRequirementDelayReadC:
			if i < 0 || len(in.capacityRequirement) <= i {
				in.errorC <- errors.New("i out of range")
			} else {
				in.capacityRequirement_i = i
			}
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
	for _, pi := range in.pipelineM {
		pi.cancel()
	}
}
