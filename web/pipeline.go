package web

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/util"
)

type pipelineChannelGroup struct {
	dataC      chan<- sub.PipelineGroup
	bidC       chan<- sub.BidGroup
	bidStatusC chan<- BidStatusWithPipelineId
	periodC    chan<- sub.PeriodGroup
}

type pipelineChannelGroupInternal struct {
	dataC      <-chan sub.PipelineGroup
	bidC       <-chan sub.BidGroup
	bidStatusC <-chan BidStatusWithPipelineId
	periodC    <-chan sub.PeriodGroup
}

type BidStatusWithPipelineId struct {
	Status     pipe.BidStatus
	PipelineId sgo.PublicKey
}

func createPipelinePair() (pipelineChannelGroup, pipelineChannelGroupInternal) {
	dataC := make(chan sub.PipelineGroup)
	periodC := make(chan sub.PeriodGroup)
	bidC := make(chan sub.BidGroup)
	bidStatusC := make(chan BidStatusWithPipelineId)

	return pipelineChannelGroup{
			dataC:      dataC,
			bidC:       bidC,
			periodC:    periodC,
			bidStatusC: bidStatusC,
		},
		pipelineChannelGroupInternal{
			dataC:      dataC,
			bidC:       bidC,
			periodC:    periodC,
			bidStatusC: bidStatusC,
		}

}

func (e1 external) ws_pipeline(
	clientCtx context.Context,
	errorC chan<- error,
	pipeOut pipelineChannelGroup,
) {

	list, err := e1.router.AllPipeline()
	if err != nil {
		errorC <- err
		return
	}
	for i := 0; i < len(list); i++ {
		d, err := list[i].Data()
		if err != nil {
			errorC <- err
			return
		}
		pipeOut.dataC <- sub.PipelineGroup{
			Id:     list[i].Id,
			Data:   d,
			IsOpen: true,
		}
		go e1.ws_on_pipeline(errorC, clientCtx, list[i], pipeOut)
	}
}

func (e1 external) ws_on_pipeline(
	errorC chan<- error,
	ctx context.Context,
	p pipe.Pipeline,
	pipeOut pipelineChannelGroup,
) {
	serverDoneC := e1.ctx.Done()
	doneC := ctx.Done()

	id := p.Id

	dataSub := p.OnData()
	defer dataSub.Unsubscribe()

	periodSub := p.OnPeriod()
	defer periodSub.Unsubscribe()

	bidSub := p.OnBid()
	defer bidSub.Unsubscribe()

	bidStatusSub := p.OnBidStatus(util.Zero())
	defer bidStatusSub.Unsubscribe()

	var err error
	{
		data, err := p.Data()
		if err != nil {
			loopPipelineFinish(id, pipeOut, errorC, err)
			return
		}
		pipeOut.dataC <- sub.PipelineGroup{
			Id:     id,
			Data:   data,
			IsOpen: true,
		}
	}
	{

		pr, err := p.PeriodRing()
		if err != nil {
			loopPipelineFinish(id, pipeOut, errorC, err)
			return
		}
		pipeOut.periodC <- sub.PeriodGroup{
			Id:     id,
			Data:   pr,
			IsOpen: true,
		}
	}

	{

		bl, err := p.BidList()
		if err != nil {
			loopPipelineFinish(id, pipeOut, errorC, err)
			return
		}
		pipeOut.bidC <- sub.BidGroup{
			Id:     id,
			Data:   bl,
			IsOpen: true,
		}
	}

	bsErrorC := make(chan error, 1)
	go loopWriteBidStatus(ctx, p, bsErrorC, pipeOut)

out:
	for {
		select {
		case err = <-bsErrorC:
			break out
		case <-serverDoneC:
			break out
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case d := <-dataSub.StreamC:
			pipeOut.dataC <- sub.PipelineGroup{
				Id:     id,
				Data:   d,
				IsOpen: true,
			}
		case err = <-periodSub.ErrorC:
			break out
		case x := <-periodSub.StreamC:
			pipeOut.periodC <- sub.PeriodGroup{
				Id:     id,
				Data:   x,
				IsOpen: true,
			}
		case err = <-bidSub.ErrorC:
			break out
		case x := <-bidSub.StreamC:
			pipeOut.bidC <- sub.BidGroup{
				Id:     id,
				Data:   x,
				IsOpen: true,
			}
		case err = <-bidStatusSub.ErrorC:
			break out
		case x := <-bidStatusSub.StreamC:
			pipeOut.bidStatusC <- BidStatusWithPipelineId{
				PipelineId: p.Id,
				Status:     x,
			}
		}
	}
	if err != nil {
		loopPipelineFinish(id, pipeOut, errorC, err)
	}
}

func loopWriteBidStatus(
	ctx context.Context,
	p pipe.Pipeline,
	errorC chan<- error,
	pipeOut pipelineChannelGroup,
) {
	doneC := ctx.Done()
	list, err := p.AllBidStatus(util.Zero())
	if err != nil {
		errorC <- err
		return
	}

	for i := 0; i < len(list); i++ {
		select {
		case <-doneC:
			return
		case pipeOut.bidStatusC <- BidStatusWithPipelineId{
			PipelineId: p.Id,
			Status:     list[i],
		}:
		}
	}

}

func loopPipelineFinish(
	id sgo.PublicKey,
	pipeOut pipelineChannelGroup,
	errorC chan<- error,
	err error,
) {
	pipeOut.dataC <- sub.PipelineGroup{
		Id:     id,
		IsOpen: false,
	}
	pipeOut.periodC <- sub.PeriodGroup{
		Id:     id,
		IsOpen: false,
	}
	pipeOut.bidC <- sub.BidGroup{
		Id:     id,
		IsOpen: false,
	}
	errorC <- err
}
