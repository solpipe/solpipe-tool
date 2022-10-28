package validator

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

type internal struct {
	ctx            context.Context
	errorC         chan<- error
	data           *cba.ValidatorManager
	validatorHome  *sub2.SubHome[cba.ValidatorManager]
	stakeRatioHome *sub2.SubHome[StakeStatus]
	receiptHome    *sub2.SubHome[rpt.ReceiptWithData]
	totalStake     uint64
	activatedStake uint64
	receipts       map[string]*receiptHolder // map payouts->
	deletePayoutC  chan<- sgo.PublicKey
	updateReceiptC chan<- cba.Receipt   // id=payout
	deleteReceiptC chan<- sgo.PublicKey // id=payout
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	id sgo.PublicKey,
	data *cba.ValidatorManager,
	validatorHome *sub2.SubHome[cba.ValidatorManager],
	activatedStakeHome sub2.Subscription[ntk.VoteStake],
	totalStatkeHome sub2.Subscription[ntk.VoteStake],
	stakeRatioHome *sub2.SubHome[StakeStatus],
	receiptHome *sub2.SubHome[rpt.ReceiptWithData],
) {
	defer cancel()
	var err error

	errorC := make(chan error, 1)
	doneC := ctx.Done()
	updateReceiptC := make(chan cba.Receipt, 10)
	deleteReceiptC := make(chan sgo.PublicKey, 1)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.data = data
	in.activatedStake = 0
	in.totalStake = 0
	in.validatorHome = validatorHome
	in.stakeRatioHome = stakeRatioHome
	in.receiptHome = receiptHome
	in.receipts = make(map[string]*receiptHolder)
	in.updateReceiptC = updateReceiptC
	in.deleteReceiptC = deleteReceiptC

	defer validatorHome.Close()
	defer stakeRatioHome.Close()
	defer receiptHome.Close()

out:
	for {
		select {
		case id := <-receiptHome.DeleteC:
			receiptHome.Delete(id)
		case r := <-receiptHome.ReqC:
			receiptHome.Receive(r)
		case id := <-stakeRatioHome.DeleteC:
			stakeRatioHome.Delete(id)
		case r := <-stakeRatioHome.ReqC:
			stakeRatioHome.Receive(r)
		case err = <-activatedStakeHome.ErrorC:
			break out
		case err = <-totalStatkeHome.ErrorC:
			break out
		case d := <-activatedStakeHome.StreamC:
			in.activatedStake = d.ActivatedStake
			in.stakeRatioHome.Broadcast(StakeStatus{
				Activated: in.activatedStake,
				Total:     in.totalStake,
			})
		case d := <-totalStatkeHome.StreamC:
			in.totalStake = d.ActivatedStake
			in.stakeRatioHome.Broadcast(StakeStatus{
				Activated: in.activatedStake,
				Total:     in.totalStake,
			})
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case id := <-in.validatorHome.DeleteC:
			in.validatorHome.Delete(id)
		case x := <-in.validatorHome.ReqC:
			in.validatorHome.Receive(x)
		case d := <-updateReceiptC:
			log.Debugf("validator receipt update: %+v", d)
			in.on_receipt_update(d)
		case id := <-deleteReceiptC:
			rh, present := in.receipts[id.String()]
			if present {
				delete(in.receipts, id.String())
				rh.cancel()
			}
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	// TODO: send updates?
}
