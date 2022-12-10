package validator

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type listenPipelineInternal struct {
	ctx        context.Context
	errorC     chan<- error
	pipeline   pipe.Pipeline
	newPayoutC chan<- payoutWithPipeline
}

// A pipeline has been selected, so listen for new Payout periods.
func loopListenPipeline(
	ctx context.Context,
	errorC chan<- error,
	pipeline pipe.Pipeline,
	newPayoutC chan<- payoutWithPipeline,
) {
	var err error
	doneC := ctx.Done()
	log.Debugf("on listen pipeline=%s", pipeline.Id.String())

	pi := new(listenPipelineInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.pipeline = pipeline

	pi.newPayoutC = newPayoutC

	payoutSub := pi.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	newPwdStreamC := make(chan pipe.PayoutWithData)

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			pi.on_payout(pwd)
		case pwd := <-newPwdStreamC:
			pi.on_payout(pwd)
		}
	}

	if err != nil {
		log.Debug(err)
		errorC <- err
	}
}

func (pi *listenPipelineInternal) on_payout(pwd pipe.PayoutWithData) {
	doneC := pi.ctx.Done()
	select {
	case <-doneC:
	case pi.newPayoutC <- payoutWithPipeline{
		pwd:      pwd,
		pipeline: pi.pipeline,
		t:        time.Now(),
	}:
	}
}
