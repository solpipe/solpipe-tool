package validator

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	spt "github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type payoutWithPipeline struct {
	pipeline pipe.Pipeline
	pwd      pipe.PayoutWithData
	t        time.Time
}

func (in *internal) on_payout(pp payoutWithPipeline) {
	pwd := pp.pwd
	log.Debugf("new payout(%s)=%+v", pwd.Id.String(), pwd.Data)
	list, err := pwd.Payout.Receipt()
	if err != nil {
		in.errorC <- err
		return
	}
	if 0 < len(list) {
		log.Debug("we already have a receipt")
		return
	}
	log.Debugf("have list=%d;  d=%+v", len(list), list)
	ctxC, cancel := context.WithCancel(in.ctx)
	in.receiptAttempt[pwd.Payout.Id.String()] = cancel
	go loopReceiptOpen(
		ctxC,
		in.errorC,
		in.scriptWrapper,
		in.controller.Id(),
		pwd.Payout.Id,
		pp.pipeline.Id,
		in.validator.Id,
		in.config.Admin,
		in.deleteReceiptAttemptC,
	)

}

func loopReceiptOpen(
	ctx context.Context,
	errorC chan<- error,
	scriptWrapper spt.Wrapper,
	controllerId sgo.PublicKey,
	payoutId sgo.PublicKey,
	pipelineId sgo.PublicKey,
	validatorId sgo.PublicKey,
	admin sgo.PrivateKey,
	deleteC chan<- sgo.PublicKey,
) {
	doneC := ctx.Done()
	receiptC := make(chan sgo.PublicKey, 1)
	log.Debugf("attempting to create receipt for payout=%s", payoutId.String())
	defer func() {
		select {
		case <-doneC:
		case deleteC <- payoutId:
		}
	}()
	err := scriptWrapper.Send(ctx, 5, 30*time.Second, func(script *spt.Script) error {
		err2 := script.SetTx(admin)
		if err2 != nil {
			return err2
		}
		r, err2 := script.ValidatorSetPipeline(
			controllerId,
			payoutId,
			pipelineId,
			validatorId,
			admin,
		)
		if err2 != nil {
			return err2
		}
		err2 = script.FinishTx(true)
		if err2 != nil {
			return err2
		}
		receiptC <- r
		return nil
	})
	if err != nil {
		// Custom(6032)
		select {
		case <-doneC:
		case errorC <- err:
		}
	} else {
		select {
		case <-doneC:
		case receiptId := <-receiptC:
			log.Debugf("created receipt=%s", receiptId.String())
		}
	}
}
