package validator

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
)

type adminExternal struct {
	pba.UnimplementedValidatorServer
	agent Agent
}

func (e1 Agent) AttachAdmin(s *grpc.Server) error {

	a := adminExternal{agent: e1}
	pba.RegisterValidatorServer(s, a)

	return nil
}

func (a adminExternal) GetDefault(
	ctx context.Context,
	req *pba.Empty,
) (*pba.ValidatorSettings, error) {
	log.Debug("validator get - 1")
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan *pba.ValidatorSettings, 1)
	var err error
	select {
	case <-doneC:
		err = errors.New("canceled")
	case a.agent.internalC <- func(in *internal) {
		if in.settings == nil {
			errorC <- errors.New("no settings")
			return
		} else {
			errorC <- nil
			ansC <- util.CopyValidatorSettings(in.settings)
		}
	}:
	}
	log.Debug("validator get - 2")
	if err != nil {
		return nil, err
	}
	log.Debug("validator get - 3")
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return nil, err
	}
	return <-ansC, nil
}

func (a adminExternal) SetDefault(
	ctx context.Context,
	req *pba.ValidatorSettings,
) (*pba.ValidatorSettings, error) {

	log.Debug("validator settings - 1")
	var err error
	if len(req.PipelineId) == 0 {
		log.Debug("validator settings - 2")
		err = a.pipeline_blank(ctx)
		if err != nil {
			return nil, err
		} else {
			log.Debug("validator settings - 3")
			return a.GetDefault(ctx, &pba.Empty{})
		}
	}
	// the pipeline is not blank
	pipelineId, err := sgo.PublicKeyFromBase58(req.PipelineId)
	if err != nil {
		return nil, err
	}
	pipeline, err := a.agent.router.PipelineById(pipelineId)
	if err != nil {
		return nil, err
	}
	log.Debugf("validator settings - 4 - pipeline=%s", pipelineId.String())
	err = a.pipeline_set(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	return a.GetDefault(ctx, &pba.Empty{})
}

func (a adminExternal) pipeline_set(ctx context.Context, pipeline pipe.Pipeline) error {
	doneC := a.agent.ctx.Done()
	errorC := make(chan error, 1)
	err := a.send_cb(ctx, func(in *internal) {
		errorC <- in.pipeline_set(pipeline)
	})
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return err
	}
	return nil
}

// Set the pipeline. Set up the listeners.
func (in *internal) pipeline_set(pipeline pipe.Pipeline) error {
	log.Debug("validator pipeline set - 1 - id=%s", pipeline.Id.String())
	var err error
	if in.hasPipeline {
		err = in.pipeline_blank()
		if err != nil {
			return err
		}
	}
	in.hasPipeline = true
	in.pipeline = pipeline
	// the SetValidator instruction is executed once a Payout account has been created
	in.payoutScannerCtx, in.payoutScannerCancel = context.WithCancel(in.ctx)
	go loopListenPeriod(
		in.payoutScannerCtx,
		in.errorC,
		in.config,
		in.controller.SlotHome(),
		in.router,
		in.pipeline,
		in.validator,
	)
	in.settings.PipelineId = pipeline.Id.String()

	// set pipeline

	return nil
}

func (a adminExternal) pipeline_blank(ctx context.Context) error {
	doneC := a.agent.ctx.Done()
	errorC := make(chan error, 1)
	err := a.send_cb(ctx, func(in *internal) {
		errorC <- in.pipeline_blank()
	})
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return err
	}
	return nil
}

func (in *internal) pipeline_blank() error {
	in.hasPipeline = false
	if in.payoutScannerCtx != nil {
		in.payoutScannerCancel()
		in.payoutScannerCtx = nil
	}
	return nil
}
