package pipeline

import (
	"context"
	"math/big"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type internal struct {
	ctx               context.Context
	errorC            chan<- error
	finishPayoutC     chan<- payoutFinish
	id                sgo.PublicKey
	data              *cba.Pipeline
	refundInfo        *refundInfo
	periods           *cba.PeriodRing
	allottedTps       *big.Float
	activatedStake    *big.Int
	totalStake        *big.Int
	networkStatus     *ntk.NetworkStatus
	payoutById        map[string]*ll.Node[PayoutWithData]
	updatePayoutC     chan<- cba.Payout
	activePayout      *ll.Node[PayoutWithData] // what payout/period does ring.Start point to
	payoutLinkedList  *ll.Generic[PayoutWithData]
	pipelineHome      *dssub.SubHome[cba.Pipeline]
	periodHome        *dssub.SubHome[cba.PeriodRing]
	claimHome         *dssub.SubHome[cba.Claim]
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
	Id     sgo.PublicKey
	Payout pyt.Payout
	Data   cba.Payout
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
	rf *cba.Refunds,
	pipelineHome *dssub.SubHome[cba.Pipeline],
	periodHome *dssub.SubHome[cba.PeriodRing],
	claimHome *dssub.SubHome[cba.Claim],
	payoutHome *dssub.SubHome[PayoutWithData],
	validatorHome *dssub.SubHome[ValidatorUpdate],
	stakeStatusHome *dssub.SubHome[sub.StakeUpdate],
	network ntk.Network,
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
	in.claimHome = claimHome
	defer claimHome.Close()
	in.payoutHome = payoutHome
	defer payoutHome.Close()
	in.validatorHome = validatorHome
	defer validatorHome.Close()
	in.stakeStatusHome = stakeStatusHome
	defer stakeStatusHome.Close()
	netSub := network.OnNetworkStats()
	defer netSub.Unsubscribe()

	in.networkStatus = &ntk.NetworkStatus{
		WindowSize:                       1,
		AverageTransactionsPerBlock:      0,
		AverageTransactionsPerSecond:     0,
		AverageTransactionsSizePerSecond: 0,
	}

	in.on_period(*pr)
	in.on_refund(*rf)

	err = in.init()
	if err != nil {
		in.errorC <- err
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case u := <-updateValidatorStakeC:
			// we must account for a situation in which
			// prior receipt account closes after
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
		case err = <-netSub.ErrorC:
			break out
		case ns := <-netSub.StreamC:
			in.on_network_stats(ns)
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

func (in *internal) init() error {
	err := in.init_refund()
	if err != nil {
		return err
	}
	return nil
}
