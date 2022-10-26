package receipt

import (
	"context"

	cba "github.com/solpipe/cba"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx           context.Context
	cancel        context.CancelFunc
	data          *cba.Receipt
	errorC        chan<- error
	stakers       map[string]skr.Staker
	receiptHome   *sub2.SubHome[cba.Receipt]
	updateStakerC chan<- sub.StakeGroup
	deleteStakerC chan<- sgo.PublicKey
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	data *cba.Receipt,
	dataC <-chan cba.Receipt,
	receiptHome *sub2.SubHome[cba.Receipt],
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	updateStakerC := make(chan sub.StakeGroup, 1)
	deleteStakerC := make(chan sgo.PublicKey, 1)

	in := new(internal)
	in.ctx = ctx
	in.cancel = cancel
	in.data = data
	in.errorC = errorC
	in.stakers = make(map[string]skr.Staker)
	in.receiptHome = receiptHome
	in.updateStakerC = updateStakerC
	in.deleteStakerC = deleteStakerC

	defer receiptHome.Close()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case d := <-dataC:
			in.on_data(d)
		case req := <-internalC:
			req(in)
		case id := <-receiptHome.DeleteC:
			receiptHome.Delete(id)
		case d := <-receiptHome.ReqC:
			receiptHome.Receive(d)
		case id := <-deleteStakerC:
			delete(in.stakers, id.String())
		case d := <-updateStakerC:
			log.Debugf("update stake id=%s; new stake=%d", d.Id.String(), d.Data.DelegatedStake)
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	in.cancel()
	// receipt is finished, so close all stakers
	for _, s := range in.stakers {
		s.Close()
	}
}