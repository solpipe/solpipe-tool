package admin

import (
	"context"
	"errors"
	"io"
	"math"

	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"

	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
)

type Server struct {
	pba.UnimplementedPipelineServer
	ctx       context.Context
	internalC chan<- func(*internal)
	reqLogC   chan<- sub.ResponseChannel[*pba.LogLine]
	pipeline  pipe.Pipeline
}

func Attach(
	ctx context.Context,
	grpcServer *grpc.Server,
	initialSettings *pipe.PipelineSettings,
	configFilePath string,
	periodSettingsC chan<- *pba.PeriodSettings,
	rateSettingsC chan<- *pba.RateSettings,
	pipeline pipe.Pipeline,
) (<-chan error, error) {
	log.Debug("creating owner grpc server")
	signalC := make(chan error, 1)
	internalC := make(chan func(*internal), 10)
	homeLog := sub.CreateSubHome[*pba.LogLine]()
	reqLogC := homeLog.ReqC

	e1 := Server{
		ctx:       ctx,
		internalC: internalC,
		reqLogC:   reqLogC,
		pipeline:  pipeline,
	}

	if initialSettings == nil {
		return signalC, errors.New("no initial settings")
	}

	go loopInternal(
		ctx,
		internalC,
		homeLog,
		initialSettings,
		configFilePath,
		periodSettingsC,
		rateSettingsC,
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
	// TODO: change
	if req.TickSize == 0 {
		req.TickSize = 1
	}
	if req.TickSize == 0 || math.MaxUint16 <= req.TickSize {
		return nil, errors.New("tick size out of range")
	}
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	var err error
	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {

		old := in.periodSettings
		in.periodSettings = newSettings
		err2 := in.config_save()
		errorC <- err2
		if err2 != nil {
			in.periodSettings = old
		} else {
			in.settings_change()
		}
	}:
	}
	if err != nil {
		return nil, err
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return nil, err
	}
	return newSettings, nil
}

func (in *internal) settings_change() {
	log.Debug("admin settings have changed")
	in.on_period_settings_update()
	in.on_rate_settings_update()
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
