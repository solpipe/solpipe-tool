package validator

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

func (in *internal) on_settings(newSettings *pba.ValidatorSettings) error {
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
	var pipeline pipe.Pipeline
	var newPipelineId *sgo.PublicKey
	if 0 < len(newSettings.PipelineId) {
		newPipelineId = new(sgo.PublicKey)
		*newPipelineId, err = sgo.PublicKeyFromBase58(newSettings.PipelineId)
		if err != nil {
			log.Debug(err)
			return err
		}
		pipeline, err = in.router.PipelineById(*newPipelineId)
		if err != nil {
			return err
		}
	} else {
		newPipelineId = nil
	}

	var isPipelineDifferent bool
	if newPipelineId == nil && in.selectedPipeline == nil {
		isPipelineDifferent = false
	} else if in.selectedPipeline == nil {
		isPipelineDifferent = true
	} else if newPipelineId.Equals(in.selectedPipeline.p.Id) {
		isPipelineDifferent = false
	} else {
		isPipelineDifferent = true
	}

	if isPipelineDifferent {
		if in.selectedPipeline != nil {
			in.selectedPipeline.cancel()
			delete(in.pipelineM, in.selectedPipeline.p.Id.String())
		}
		if newPipelineId != nil {
			log.Debugf("validator selecting payouts from pipeline=%s", newPipelineId.String())
			in.selectedPipeline = in.on_pipeline(pipeline)
		}
	} else {
		log.Debug("no change in pipeline")
	}

	in.on_settings_lookahead()

	return nil
}

func (in *internal) on_settings_lookahead() {
	doneC := in.ctx.Done()
	for _, pi := range in.pipelineM {
		select {
		case <-doneC:
			return
		case pi.lookAheadC <- in.settings.GetLookahead():
		}
	}
}
