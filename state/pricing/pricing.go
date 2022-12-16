package pricing

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Pricing struct {
	ctx       context.Context
	router    rtr.Router
	internalC chan<- func(*internal)
	bidder    sgo.PublicKey
	handler   ts.Handle
}

func Create(
	ctx context.Context,
	router rtr.Router,
	bidder sgo.PublicKey,
	handler ts.Handle,
) Pricing {
	internalC := make(chan func(*internal))
	go loopInternal(
		ctx,
		internalC,
		router,
		bidder,
		handler,
	)

	return Pricing{
		ctx:       ctx,
		router:    router,
		internalC: internalC,
		bidder:    bidder,
		handler:   handler,
	}
}

func (e1 Pricing) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	return signalC
}

func (e1 Pricing) CapacitySet(curve []CapacityPoint) error {
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		return errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.capacity_set(curve)
	}:
	}
	return nil
}

func (in *internal) capacity_set(curve []CapacityPoint) {
	in.capacityRequirement = curve
}

// set the
func (e1 Pricing) PipelineMixSet(mix map[string]float64) error {

	doneC := e1.ctx.Done()
	total := float64(0)
	var err error
	for k, v := range mix {
		if v < 0 {
			return errors.New("cannot have negative tps value")
		} else if v == 0 {
			delete(mix, k)
		} else if 1 < v {
			return errors.New("cannot have weight greater than 1 for a pipeline")
		} else {
			total += v
			_, err = sgo.PublicKeyFromBase58(k)
			if err != nil {
				return err
			}
		}
	}
	if total != 1 {
		return errors.New("weights do not sum to 1")
	}

	select {
	case <-doneC:
		return errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.pipeline_mix_set(mix)
	}:
	}
	return nil
}

func (in *internal) pipeline_mix_set(mix map[string]float64) {
	in.pipelineMixM = mix
}
