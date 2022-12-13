package pricing

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type bidUpdate struct {
	pipelineId sgo.PublicKey
	payoutId   sgo.PublicKey
	bs         pyt.BidStatus
}

type periodUpdate struct {
	pipelineId sgo.PublicKey
	payoutId   sgo.PublicKey
	period     cba.Period
	bs         pyt.BidStatus
}

func loopPayout(
	ctx context.Context,
	pipeline pipe.Pipeline,
	pwd pipe.PayoutWithData,
	errorC chan<- error,
	bidC chan<- bidUpdate,
) {
	doneC := ctx.Done()
	var err error
	//payout = pwd.Payout

	bidSub := pwd.Payout.OnBidStatus()
	defer bidSub.Unsubscribe()
	var bs pyt.BidStatus
	bs, err = pwd.Payout.BidStatus()
	if err != nil {
		errorC <- err
		return
	}
	select {
	case <-doneC:
		return
	case bidC <- bidUpdate{
		pipelineId: pipeline.Id,
		payoutId:   pwd.Id,
		bs:         bs,
	}:
	}
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-bidSub.ErrorC:
			break out
		case bs = <-bidSub.StreamC:
			select {
			case <-doneC:
				break out
			case bidC <- bidUpdate{
				pipelineId: pipeline.Id,
				payoutId:   pwd.Id,
				bs:         bs,
			}:
			}
		}
	}

	if err != nil {
		select {
		case <-time.After(10 * time.Second):
		case errorC <- err:
		}
	}
}
