package admin

import (
	"context"
	"errors"
	"io"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	"github.com/solpipe/solpipe-tool/state/pipeline"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
)

type Server struct {
	pba.UnimplementedPipelineServer
	ctx        context.Context
	controller ctr.Controller
	internalC  chan<- func(*internal)
	reqLogC    chan<- sub.ResponseChannel[*pba.LogLine]
	pipeline   pipe.Pipeline
}

func Attach(
	ctx context.Context,
	grpcServer *grpc.Server,
	router rtr.Router,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	pipelineId sgo.PublicKey,
	admin sgo.PrivateKey,
	initialSettings *pipeline.PipelineSettings,
) (<-chan error, error) {
	log.Debug("creating owner grpc server")
	signalC := make(chan error, 1)
	controller := router.Controller
	slotHome := controller.SlotHome()
	internalC := make(chan func(*internal), 10)
	homeLog := sub.CreateSubHome[*pba.LogLine]()
	reqLogC := homeLog.ReqC
	pipeline, err := router.PipelineById(pipelineId)
	if err != nil {
		return signalC, err
	}
	e1 := Server{
		ctx:        ctx,
		controller: controller,
		internalC:  internalC,
		reqLogC:    reqLogC,
		pipeline:   pipeline,
	}

	if initialSettings == nil {
		return signalC, errors.New("no initial settings")
	}

	pr, err := pipeline.PeriodRing()
	if err != nil {
		return signalC, err
	}
	prList, err := util.GetLinkedListFromPeriodRing(&pr)
	if err != nil {
		return signalC, err
	}

	go loopInternal(
		ctx,
		internalC,
		rpcClient,
		wsClient,
		admin,
		controller,
		router,
		pipeline,
		slotHome,
		homeLog,
		prList,
		initialSettings,
	)

	pba.RegisterPipelineServer(grpcServer, e1)

	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}

	return signalC, nil
}

func (e1 Server) GetPeriod(ctx context.Context, req *pba.Empty) (*pba.PeriodSettings, error) {
	var err error
	errorC := make(chan error, 1)
	settingsC := make(chan *pba.PeriodSettings, 1)
	e1.internalC <- func(in *internal) {
		if in.periodSettings == nil {
			errorC <- errors.New("no settings")
			return
		} else {
			errorC <- nil
			settingsC <- util.CopyPeriodSettings(in.periodSettings)
		}

	}

	doneC := ctx.Done()
	select {
	case <-doneC:
		err = errors.New("no settings")
	case err = <-errorC:
	}
	if err != nil {
		return nil, err
	}
	return <-settingsC, nil
}

func (e1 Server) SetPeriod(ctx context.Context, req *pba.PeriodSettings) (*pba.PeriodSettings, error) {
	newSettings := util.CopyPeriodSettings(req)
	e1.internalC <- func(in *internal) {
		in.periodSettings = newSettings
	}
	return newSettings, nil
}

func (e1 Server) GetLogStream(req *pba.Empty, stream pba.Validator_GetLogStreamServer) error {
	ctx := stream.Context()
	sub := sub.SubscriptionRequest(e1.reqLogC, func(x *pba.LogLine) bool { return true })
	doneC := ctx.Done()
	var err error
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-sub.ErrorC:
			break out
		case d := <-sub.StreamC:
			err = stream.Send(d)
			if err == io.EOF {
				err = nil
				break out
			} else if err != nil {
				break out
			}
		}
	}
	return err
}
