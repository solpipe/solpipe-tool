package web

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type pipelineChannelGroup struct {
	dataC   chan<- sub.PipelineGroup
	bidC    chan<- sub.BidGroup
	periodC chan<- sub.PeriodGroup
}

type pipelineChannelGroupInternal struct {
	dataC   <-chan sub.PipelineGroup
	bidC    <-chan sub.BidGroup
	periodC <-chan sub.PeriodGroup
}

func createPipelinePair() (pipelineChannelGroup, pipelineChannelGroupInternal) {
	dataC := make(chan sub.PipelineGroup)
	periodC := make(chan sub.PeriodGroup)
	bidC := make(chan sub.BidGroup)

	return pipelineChannelGroup{
			dataC:   dataC,
			bidC:    bidC,
			periodC: periodC,
		},
		pipelineChannelGroupInternal{
			dataC:   dataC,
			bidC:    bidC,
			periodC: periodC,
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

out:
	for {
		select {
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
		}
	}
	if err != nil {
		loopPipelineFinish(id, pipeOut, errorC, err)
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
