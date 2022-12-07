package validator

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

func (in *internal) settings_change(newSettings *pba.ValidatorSettings) error {
	var err error
	log.Debug("settings have change")
	if newSettings == nil {
		return errors.New("no settings")
	}
	oldSettings := in.settings
	in.settings = newSettings
	err = in.config_save()
	if err != nil {
		log.Debug(err)
		in.settings = oldSettings
		return err
	}

	var newPipelineId *sgo.PublicKey
	if 0 < len(newSettings.PipelineId) {
		newPipelineId = new(sgo.PublicKey)
		*newPipelineId, err = sgo.PublicKeyFromBase58(newSettings.PipelineId)
		if err != nil {
			log.Debug(err)
			return err
		}
	} else {
		newPipelineId = nil
	}

	var isPipelineDifferent bool
	if newPipelineId == nil && in.pipeline == nil {
		isPipelineDifferent = false
	} else if in.pipeline == nil {
		isPipelineDifferent = true
	} else if newPipelineId.Equals(in.pipeline.Id) {
		isPipelineDifferent = false
	} else {
		isPipelineDifferent = true
	}

	if isPipelineDifferent {
		if in.pipeline != nil {
			in.pipelineCancel()
		}
		if newPipelineId != nil {
			log.Debugf("validator selecting payouts from pipeline=%s", newPipelineId.String())
			in.pipelineCtx, in.pipelineCancel = context.WithCancel(in.ctx)
			in.pipeline = new(pipe.Pipeline)
			*in.pipeline, err = in.router.PipelineById(*newPipelineId)
			if err != nil {
				in.pipelineCancel()
				in.pipeline = nil
				return err
			}
			go loopListenPipeline(
				in.pipelineCtx,
				in.errorC,
				in.config,
				in.controller.SlotHome(),
				in.router,
				*in.pipeline,
				in.validator,
			)
		}
	} else {
		log.Debug("no change in pipeline")
	}

	return nil
}
