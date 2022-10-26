package validator

import (
	"context"
	"time"

	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	val "github.com/solpipe/solpipe-tool/state/validator"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	config           relay.Configuration
	pipeline         pipe.Pipeline
	stakeShare       float64
	networkTps       float64
	txCountInPeriod  float64
	allowedTps       float64
	currentTps       float64
	readTx           bool
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	txC <-chan *submitInfo,
	internalC <-chan func(*internal),
	validator val.Validator,
	network ntk.Network,
	config relay.Configuration,
) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.config = config
	in.stakeShare = 0
	in.networkTps = 0
	in.txCountInPeriod = 0
	in.readTx = true
	in.currentTps = 0
	in.allowedTps = 0

	validatorSub := validator.OnStake()
	defer validatorSub.Unsubscribe()

	networkSub := network.OnNetworkStats()
	defer networkSub.Unsubscribe()

	plusBuffer := float64(110) / float64(100)
	interval := 10 * time.Second
	boxStart := float64(time.Now().Unix())
	nextBoxC := time.After(interval)

out:
	for {
		if in.readTx {
			select {
			case si := <-txC:
				in.process(si)
				in.txCountInPeriod++
				if in.allowedTps <= (in.txCountInPeriod / (float64(time.Now().Unix()) - boxStart)) {
					in.readTx = false
				}
			case <-nextBoxC:
				in.txCountInPeriod = 0
				in.readTx = true
				nextBoxC = time.After(interval)
			case err = <-networkSub.ErrorC:
				break out
			case n := <-networkSub.StreamC:
				// add 10% buffer
				in.networkTps = n.AverageTransactionsPerSecond * plusBuffer
				in.allowedTps = in.networkTps * in.stakeShare
			case err = <-validatorSub.ErrorC:
				break out
			case s := <-validatorSub.StreamC:
				if s.Total == 0 {
					in.stakeShare = 0
				} else {
					in.stakeShare = float64(s.Activated) / float64(s.Total)
					in.allowedTps = in.networkTps * in.stakeShare
				}
			case <-doneC:
				break out
			case err = <-errorC:
				break out
			case req := <-internalC:
				req(in)
			}
		} else {
			select {
			case <-nextBoxC:
				in.txCountInPeriod = 0
				in.readTx = true
				nextBoxC = time.After(interval)
			case err = <-networkSub.ErrorC:
				break out
			case n := <-networkSub.StreamC:
				in.networkTps = n.AverageTransactionsPerSecond * plusBuffer
				in.allowedTps = in.networkTps * in.stakeShare
			case err = <-validatorSub.ErrorC:
				break out
			case s := <-validatorSub.StreamC:
				if s.Total == 0 {
					in.stakeShare = 0
				} else {
					in.stakeShare = float64(s.Activated) / float64(s.Total)
					in.allowedTps = in.networkTps * in.stakeShare
				}
			case <-doneC:
				break out
			case err = <-errorC:
				break out
			case req := <-internalC:
				req(in)
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
}

func (e1 external) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}
