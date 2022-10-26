package pipeline

import (
	"context"
	"math/big"
	"time"

	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx               context.Context
	cancel            context.CancelFunc
	errorC            chan<- error
	finishPayoutC     chan<- payoutFinish
	id                sgo.PublicKey
	data              *cba.Pipeline
	bids              *cba.BidList
	periods           *cba.PeriodRing
	allottedTps       *big.Float
	activatedStake    *big.Int
	totalStake        *big.Int
	payoutById        map[string]*ll.Node[PayoutWithData]
	updatePayoutC     chan<- cba.Payout
	activePayout      *ll.Node[PayoutWithData] // what payout/period does ring.Start point to
	payoutLinkedList  *ll.Generic[PayoutWithData]
	pipelineHome      *dssub.SubHome[cba.Pipeline]
	periodHome        *dssub.SubHome[cba.PeriodRing]
	bidHome           *dssub.SubHome[cba.BidList]
	payoutHome        *dssub.SubHome[PayoutWithData]
	validatorHome     *dssub.SubHome[ValidatorUpdate]
	stakeStatusHome   *dssub.SubHome[sub.StakeUpdate]
	validatorStakeSub map[string]*validatorStakeSub // map vote id -> stake sub
}

type payoutFinish struct {
	id  sgo.PublicKey
	err error
}

type PayoutWithData struct {
	IsEmpty bool
	Id      sgo.PublicKey
	Payout  pyt.Payout
	Data    cba.Payout
}

const PROXY_MAX_CONNECTION_ATTEMPT = 10
const PROXY_RECONNECT_SLEEP = 10 * time.Second

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	updateValidatorStakeC <-chan validatorStakeUpdate,
	id sgo.PublicKey,
	data *cba.Pipeline,
	pr *cba.PeriodRing,
	bl *cba.BidList,
	pipelineHome *dssub.SubHome[cba.Pipeline],
	periodHome *dssub.SubHome[cba.PeriodRing],
	bidHome *dssub.SubHome[cba.BidList],
	payoutHome *dssub.SubHome[PayoutWithData],
	validatorHome *dssub.SubHome[ValidatorUpdate],
	stakeStatusHome *dssub.SubHome[sub.StakeUpdate],
) {
	defer cancel()
	var err error

	errorC := make(chan error, 1)
	doneC := ctx.Done()
	updatePayoutC := make(chan cba.Payout, 10)
	finishPayoutC := make(chan payoutFinish, 1)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.finishPayoutC = finishPayoutC
	in.id = id
	in.data = data
	in.allottedTps = big.NewFloat(0)
	in.activatedStake = big.NewInt(0)
	in.totalStake = big.NewInt(1)
	in.payoutLinkedList = ll.CreateGeneric[PayoutWithData]()
	in.payoutById = make(map[string]*ll.Node[PayoutWithData])
	in.pipelineHome = pipelineHome
	defer pipelineHome.Close()
	in.periodHome = periodHome
	defer periodHome.Close()
	in.updatePayoutC = updatePayoutC
	in.bidHome = bidHome
	defer bidHome.Close()
	in.payoutHome = payoutHome
	defer payoutHome.Close()
	in.validatorHome = validatorHome
	defer validatorHome.Close()
	in.stakeStatusHome = stakeStatusHome
	defer stakeStatusHome.Close()

	in.on_period(*pr)
	in.on_bid(*bl)

out:
	for {
		select {
		case u := <-updateValidatorStakeC:
			// we must account for a situation in which
			// prior receipt accoutn closes after
			// the next receipt account is created
			// and we cannot delete a validator from an expiring receipt
			// after adding a validator from an new receipt
			x, present := in.validatorStakeSub[u.vote.String()]
			if present {
				oldStatus := *x.status
				var newStatus val.StakeStatus
				hasDeleted := false
				if x.finish <= u.finish {
					if u.status.Activated == 0 && u.status.Total == 0 {
						delete(in.validatorStakeSub, u.vote.String())
						hasDeleted = true
					}
				}
				if !hasDeleted {
					x.status = new(val.StakeStatus)
					*x.status = u.status
					x.start = u.start
					x.finish = u.finish
					newStatus = u.status
				} else {
					newStatus = val.StakeStatus{
						Activated: 0,
						Total:     0,
					}
				}
				delta := val.DeltaStake(oldStatus, newStatus)
				in.activatedStake.Add(in.activatedStake, delta.ActiveStakeChange())
				in.totalStake.SetUint64(newStatus.Total)

				a := big.NewInt(0)
				t := big.NewInt(0)
				in.stakeStatusHome.Broadcast(sub.StakeUpdate{
					ActivatedStake: a.Set(in.activatedStake),
					TotalStake:     t.Set(in.totalStake),
				})
			}
		case x := <-in.pipelineHome.ReqC:
			in.pipelineHome.Receive(x)
		case id := <-in.pipelineHome.DeleteC:
			in.pipelineHome.Delete(id)
		case x := <-in.periodHome.ReqC:
			in.periodHome.Receive(x)
		case id := <-in.periodHome.DeleteC:
			in.periodHome.Delete(id)
		case x := <-in.bidHome.ReqC:
			in.bidHome.Receive(x)
		case id := <-in.bidHome.DeleteC:
			in.bidHome.Delete(id)
		case x := <-in.payoutHome.ReqC:
			in.payoutHome.Receive(x)
		case id := <-in.payoutHome.DeleteC:
			in.payoutHome.Delete(id)
		case d := <-updatePayoutC:
			in.on_payout_update(d)
		case id := <-validatorHome.DeleteC:
			validatorHome.Delete(id)
		case r := <-validatorHome.ReqC:
			validatorHome.Receive(r)
		case id := <-stakeStatusHome.DeleteC:
			stakeStatusHome.Delete(id)
		case r := <-stakeStatusHome.ReqC:
			stakeStatusHome.Receive(r)
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case x := <-finishPayoutC:
			node, present := in.payoutById[x.id.String()]
			if present {
				in.payoutLinkedList.Remove(node)
				delete(in.payoutById, x.id.String())
			}
		case req := <-internalC:
			req(in)
		}
	}

	if err != nil {
		log.Debug(err)
	}
}
