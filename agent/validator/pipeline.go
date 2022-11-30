package validator

import (
	"context"

	log "github.com/sirupsen/logrus"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

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

	err = in.config_save()
	if err != nil {
		return err
	}
	// set pipeline

	return nil
}
