package pricing

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (in *internal) on_pipeline(
	pipeline pipe.Pipeline,
) {
	_, present := in.pipelineM[pipeline.Id.String()]
	if present {
		return
	}
	data, err := pipeline.Data()
	if err != nil {
		in.errorC <- err
		return
	}
	ctxC, cancel := context.WithCancel(in.ctx)
	pi := &pipelineInfo{
		in:         in,
		bidder:     in.bidder,
		p:          pipeline,
		data:       data,
		cancel:     cancel,
		periodList: ll.CreateGeneric[*periodInfo](),
		stats:      new(pipelineStats),
	}

	in.pipelineM[pi.Id().String()] = pi

	go loopPipeline(
		ctxC,
		pipeline,
		in.errorC,
		in.pipelineDataC,
		in.stakeC,
		in.periodC,
		in.bidC,
	)
}

func loopPipeline(
	ctx context.Context,
	pipeline pipe.Pipeline,
	errorC chan<- error,
	dataC chan<- sub.PipelineGroup,
	stakeC chan<- stakeUpdate,
	periodC chan<- periodUpdate,
	bidC chan<- bidUpdate,
) {
	var err error
	var present bool
	doneC := ctx.Done()

	dataSub := pipeline.OnData()
	defer dataSub.Unsubscribe()
	var data cba.Pipeline

	payoutM := make(map[string]bool)
	payoutSub := pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	stakeSub := pipeline.OnRelativeStake()
	defer stakeSub.Unsubscribe()
	var relstake sub.StakeUpdate

	var pwd pipe.PayoutWithData
	{
		pwdList, err := pipeline.AllPayouts()
		if err != nil {
			select {
			case <-doneC:
			case errorC <- err:
			}
			return
		}
		for i := 0; i < len(pwdList); i++ {
			pwd = pwdList[i]
			_, present = payoutM[pwd.Id.String()]
			if !present {
				payoutM[pwd.Id.String()] = true
				go loopPayout(
					ctx,
					pipeline,
					pwd,
					errorC,
					bidC,
				)
				bs, err := pwd.Payout.BidStatus()
				if err != nil {
					errorC <- err
					return
				}
				select {
				case <-doneC:
					return
				case periodC <- periodUpdate{
					pipelineId: pipeline.Id,
					payoutId:   pwd.Id,
					period:     pwd.Data.Period,
					bs:         bs,
				}:
				}
			}
		}
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-stakeSub.ErrorC:
			break out
		case relstake = <-stakeSub.StreamC:
			select {
			case <-doneC:
				break out
			case stakeC <- stakeUpdate{
				pipelineId: pipeline.Id,
				relative:   relstake,
			}:
			}
		case err = <-dataSub.ErrorC:
			break out
		case data = <-dataSub.StreamC:
			select {
			case <-doneC:
			case dataC <- sub.PipelineGroup{
				Id:     pipeline.Id,
				Data:   data,
				IsOpen: true,
			}:
			}
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			_, present := payoutM[pwd.Id.String()]
			if !present {
				payoutM[pwd.Id.String()] = true
				go loopPayout(
					ctx,
					pipeline,
					pwd,
					errorC,
					bidC,
				)
				bs, err := pwd.Payout.BidStatus()
				if err != nil {
					errorC <- err
					return
				}
				select {
				case <-doneC:
					return
				case periodC <- periodUpdate{
					pipelineId: pipeline.Id,
					payoutId:   pwd.Id,
					period:     pwd.Data.Period,
					bs:         bs,
				}:
				}
			}
		}
	}

	if err != nil {
		select {
		case <-time.After(5 * time.Second):
		case errorC <- err:
		}
	} else {
		select {
		case <-doneC:
		case dataC <- sub.PipelineGroup{
			Id:     pipeline.Id,
			IsOpen: false,
		}:
		}
	}
}

type pipelineInfo struct {
	in         *internal
	bidder     sgo.PublicKey
	p          pipe.Pipeline
	data       cba.Pipeline
	cancel     context.CancelFunc
	periodList *ll.Generic[*periodInfo]
	stats      *pipelineStats
}

type pipelineStats struct {
	tps float64
}

// Change something in stats, but maintain
// periodInfo still references the old stats
func (pi *pipelineInfo) stats_migrate() *pipelineStats {
	newStats := new(pipelineStats)
	*newStats = *pi.stats
	oldStats := pi.stats
	pi.stats = newStats
	return oldStats
}

func (pi *pipelineInfo) Id() sgo.PublicKey {
	return pi.p.Id
}
