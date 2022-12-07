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
	ctx    context.Context
	errorC chan<- error
	//setValidatorC chan<- sgo.PublicKey // payout id
	slotHome       slot.SlotHome
	router         rtr.Router
	pipeline       pipe.Pipeline
	pipelineData   cba.Pipeline
	validator      val.Validator
	validatorData  cba.ValidatorManager
	config         rly.Configuration
	script         *script.Script
	payoutMap      map[string]*receiptInfo
	closeReceiptC  chan<- sgo.PublicKey
	deleteReceiptC chan<- sgo.PublicKey
}

// A pipeline has been selected, so listen for new Payout periods.
func loopListenPipeline(
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
	closeReceiptC := make(chan sgo.PublicKey)
	deleteReceiptC := make(chan sgo.PublicKey)
	log.Debugf("on listen pipeline=%s", pipeline.Id.String())

	pi := new(listenPeriodInternal)
	pi.ctx = ctx
	pi.errorC = errorC
	pi.closeReceiptC = closeReceiptC
	pi.deleteReceiptC = deleteReceiptC
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
	pi.payoutMap = make(map[string]*receiptInfo)
	pi.script, err = config.ScriptBuilder(pi.ctx)
	if err != nil {
		errorC <- err
		return
	}

	payoutSub := pi.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	newPwdStreamC := make(chan pipe.PayoutWithData)
	go lookupPayoutWithData(pi.ctx, pi.pipeline, pi.errorC, newPwdStreamC)

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			pi.on_payout(pwd)
		case pwd := <-newPwdStreamC:
			pi.on_payout(pwd)
		case payoutId := <-closeReceiptC:
			ri, present := pi.payoutMap[payoutId.String()]
			if present {
				pi.receipt_close(ri)
			}
		case payoutId := <-deleteReceiptC:
			ri, present := pi.payoutMap[payoutId.String()]
			if present {
				delete(pi.payoutMap, payoutId.String())
				pi.receipt_delete(ri)
			}
		}
	}

	if err != nil {
		log.Debug(err)
		errorC <- err
	}
}

func lookupPayoutWithData(
	ctx context.Context,
	p pipe.Pipeline,
	errorC chan<- error,
	pwdStreamC chan<- pipe.PayoutWithData,
) {
	doneC := ctx.Done()
	list, err := p.PayoutWithData()
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
		return
	}
	for _, pwd := range list {
		select {
		case <-doneC:
			return
		case pwdStreamC <- pwd:
		}
	}
}

// create a receipt for some payout (corresponds to a single pipeline)
func (pi *listenPeriodInternal) validator_set(payoutId sgo.PublicKey) (receiptId sgo.PublicKey, err error) {
	log.Debugf("setting validator to payout=%s", payoutId.String())
	err = pi.script.SetTx(pi.config.Admin)
	if err != nil {
		return
	}
	//var receipt sgo.PublicKey
	receiptId, err = pi.script.ValidatorSetPipeline(
		pi.pipelineData.Controller,
		payoutId,
		pi.pipeline.Id,
		pi.validator.Id,
		pi.config.Admin,
	)
	if err != nil {
		return
	}
	err = pi.script.FinishTx(true)
	if err != nil {
		return
	}
	return
}

func (pi *listenPeriodInternal) on_payout(pwd pipe.PayoutWithData) {
	_, present := pi.payoutMap[pwd.Id.String()]
	if !present {
		log.Debugf("on payout id=%s", pwd.Id.String())
		ctxC, cancel := context.WithCancel(pi.ctx)

		go loopReceipt(
			ctxC,
			cancel,
			pi.errorC,
			pi.slotHome,
			pwd.Data.Period,
			pwd,
			pi.validator.Id,
		)

		receiptId, err := pi.validator_set(pwd.Id)
		ri := &receiptInfo{
			pwd:       pwd,
			cancel:    cancel,
			receiptId: receiptId,
		}
		if err != nil {
			log.Debug(err)
			ri.cancel()
		} else {
			pi.payoutMap[ri.Id().String()] = ri
			go loopDeleteReceipt(pi.ctx, ctxC, pi.deleteReceiptC, ri.pwd.Id)
		}
	}
}

func loopDeleteReceipt(
	ctxParent context.Context,
	ctxChild context.Context,
	deleteC chan<- sgo.PublicKey,
	payoutId sgo.PublicKey,
) {
	parentC := ctxParent.Done()
	childC := ctxChild.Done()
	select {
	case <-parentC:
		// we are turning off completely
	case <-childC:
		select {
		case <-parentC:
		case deleteC <- payoutId:
		}
	}
}
