package validator

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/slot"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type listenPeriodInternal struct {
	ctx           context.Context
	errorC        chan<- error
	setValidatorC chan<- sgo.PublicKey // payout id
	slotHome      slot.SlotHome
	router        rtr.Router
	pipeline      pipe.Pipeline
	pipelineData  cba.Pipeline
	validator     val.Validator
	validatorData cba.ValidatorManager
	config        rly.Configuration
	script        *script.Script
	payoutMap     map[string]pipe.PayoutWithData
}

// A pipeline has been selected, so listen for new Payout periods.
func loopListenPeriod(
	ctx context.Context,
	errorC chan<- error,
	config rly.Configuration,
	slotHome slot.SlotHome,
	router rtr.Router,
	pipeline pipe.Pipeline,
	validator val.Validator,
) {
	var err error
	doneC := ctx.Done()
	setValidatorOnPayoutC := make(chan sgo.PublicKey, 1)

	pi := new(listenPeriodInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.setValidatorC = setValidatorOnPayoutC
	pi.config = config
	pi.slotHome = slotHome
	pi.router = router
	pi.pipeline = pipeline
	pi.pipelineData, err = pi.pipeline.Data()
	if err != nil {
		errorC <- err
		return
	}
	pi.validator = validator
	pi.validatorData, err = pi.validator.Data()
	if err != nil {
		errorC <- err
		return
	}
	pi.payoutMap = make(map[string]pipe.PayoutWithData)
	pi.script, err = config.ScriptBuilder(pi.ctx)
	if err != nil {
		errorC <- err
		return
	}

	periodSub := pi.pipeline.OnPeriod()
	defer periodSub.Unsubscribe()
	var list []pipe.PayoutWithData

	var present bool
out:
	for {
		select {
		case <-doneC:
			break out
		case payoutId := <-setValidatorOnPayoutC:
			// run the SetValidator instruction to cr
			pi.validator_set(payoutId)
		case err = <-periodSub.ErrorC:
			break out
		case <-periodSub.StreamC:
			list, err = pi.pipeline.PeriodRingWithPayout()
			if err != nil {
				break out
			}
			for _, item := range list {
				_, present = pi.payoutMap[item.Data.Pipeline.String()]
				if !present {
					pi.payoutMap[item.Data.Pipeline.String()] = item
					go loopReceipt(
						pi.ctx,
						pi.errorC,
						pi.setValidatorC,
						pi.slotHome,
						item.Data.Period,
						item,
						pi.validator.Id,
					)
				}
			}
		}
	}

	if err != nil {
		log.Debug(err)
		errorC <- err
	}
}

func (pi *listenPeriodInternal) validator_set(payoutId sgo.PublicKey) error {
	err := pi.script.SetTx(pi.config.Admin)
	if err != nil {
		return err
	}
	//var receipt sgo.PublicKey
	_, err = pi.script.ValidatorSetPipeline(
		pi.pipelineData.Controller,
		payoutId,
		pi.pipeline.Id,
		pi.validator.Id,
		pi.config.Admin,
	)
	if err != nil {
		return err
	}
	err = pi.script.FinishTx(true)
	if err != nil {
		return err
	}
	return nil
}
