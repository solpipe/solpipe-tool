package router

import (
	"context"
	"errors"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx                       context.Context
	network                   ntk.Network
	controller                ctr.Controller
	closeSignalCList          []chan<- error
	errorC                    chan<- error
	extraInternalC            chan<- func(*internal)
	rpc                       *sgorpc.Client
	ws                        *sgows.Client
	oa                        *objSub
	reqClose                  objReqClose
	l_staker                  *lookUpStaker
	l_receipt                 *lookUpReceipt
	l_validator               *lookUpValidator
	l_payout                  *lookUpPayout
	l_pipeline                *lookUpPipeline
	unmatchedPeriodRings      map[string]cba.PeriodRing
	unmatchedBidLists         map[string]cba.BidList
	unmatchedStaker           map[string]sub.StakeGroup     // set by receipt id
	unmatchedReceiptsByPayout map[string]sub.ReceiptGroup   // set by payout
	unmatchedPayouts          map[string]sub.PayoutWithData // set by pipeline id
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	network ntk.Network,
	controller ctr.Controller,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	allErrorC <-chan error,
	startErrorC chan<- error,
	reqClose objReqClose,
	validatorG sub2.Subscription[sub.ValidatorGroup],
	pipelineG sub2.Subscription[sub.PipelineGroup],
	bidListG sub2.Subscription[cba.BidList],
	periodRingG sub2.Subscription[cba.PeriodRing],
	stakerManagerG sub2.Subscription[sub.StakeGroup],
	stakerReceiptG sub2.Subscription[sub.StakerReceiptGroup],
	receiptG sub2.Subscription[sub.ReceiptGroup],
	payoutG sub2.Subscription[sub.PayoutWithData],
	subAll *sub.SubscriptionProgramGroup,
	all *sub.ProgramAllResult,
	oa *objSub,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	extraInternalC := make(chan func(*internal), 10)

	in := new(internal)
	in.ctx = ctx
	in.extraInternalC = extraInternalC
	in.network = network
	in.controller = controller
	in.errorC = errorC
	in.oa = oa
	in.reqClose = reqClose
	in.closeSignalCList = make([]chan<- error, 0)
	in.rpc = rpcClient
	in.ws = wsClient

	err = in.init(all)
	startErrorC <- err
	if err != nil {
		return
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-allErrorC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case req := <-extraInternalC:
			req(in)
		case err = <-validatorG.ErrorC:
			break out
		case d := <-validatorG.StreamC:
			log.Debug("received validator update____")
			in.on_validator(d)
		case err = <-pipelineG.ErrorC:
			break out
		case pg := <-pipelineG.StreamC:
			// received pipeline update
			log.Debug("received pipeline update____")
			in.on_pipeline(pg, in.controller.SlotHome())
		case err = <-bidListG.ErrorC:
			break out
		case list := <-bidListG.StreamC:
			// received bid list update
			log.Debug("received bid list_______")
			in.on_bid(list)
		case err = <-periodRingG.ErrorC:
			break out
		case ring := <-periodRingG.StreamC:
			// received period ring update
			log.Debug("received period ring_____")
			in.on_period(ring)
		case err = <-payoutG.ErrorC:
			break out
		case d := <-payoutG.StreamC:
			log.Debug("received payout update____; id=%s; is open=%f", d.Id.String(), d.IsOpen)
			in.on_payout(d)
		case err = <-receiptG.ErrorC:
			break out
		case d := <-receiptG.StreamC:
			log.Debug("received receipt update____")
			in.on_receipt(d)
		case err = <-stakerManagerG.ErrorC:
			break out
		case d := <-stakerManagerG.StreamC:
			log.Debug("received staker manager update____")
			in.on_stake(d)
		case err = <-stakerReceiptG.ErrorC:
			break out
		case d := <-stakerReceiptG.StreamC:
			log.Debug("received staker receipt update____")
			in.on_stake_receipt(d)
		case id := <-oa.controllerCloseC:
			if id.Equals(in.controller.Id()) {
				err = errors.New("controller has closed")
				break out
			}
		case id := <-oa.pipelineCloseC:
			delete(in.l_pipeline.byId, id.String())
		case id := <-oa.payoutCloseC:
			ref, present := in.l_payout.byId[id.String()]
			if present {
				delete(in.l_payout.byId, id.String())
				x, present := in.l_payout.byPipeline[ref.data.Pipeline.String()]
				if present {
					delete(x, ref.data.Period.Start)
				}
			}
			delete(in.l_payout.receiptWithNoPayout, id.String())
			delete(in.l_pipeline.payoutWithNoPipeline, id.String())
		case id := <-oa.receiptCloseC:
			delete(in.l_receipt.byId, id.String())
			delete(in.l_receipt.stakerWithNoReceipt, id.String())
		case id := <-oa.stakerCloseC:
			delete(in.l_staker.byId, id.String())
		case id := <-oa.validatorCloseC:
			ref, present := in.l_validator.byId[id.String()]
			if present {
				delete(in.l_validator.byId, id.String())
				delete(in.l_validator.byVote, ref.data.Vote.String())
			}
		case id := <-in.oa.controller.DeleteC:
			in.oa.controller.Delete(id)
		case id := <-in.oa.pipeline.DeleteC:
			in.oa.pipeline.Delete(id)
		case id := <-in.oa.payout.DeleteC:
			in.oa.payout.Delete(id)
		case id := <-in.oa.receipt.DeleteC:
			in.oa.receipt.Delete(id)
		case id := <-in.oa.staker.DeleteC:
			in.oa.staker.Delete(id)
		case id := <-in.oa.validator.DeleteC:
			in.oa.validator.Delete(id)
		case r := <-in.oa.controller.ReqC:
			in.oa.controller.Receive(r)
		case r := <-in.oa.pipeline.ReqC:
			in.oa.pipeline.Receive(r)
		case r := <-in.oa.payout.ReqC:
			in.oa.payout.Receive(r)
		case r := <-in.oa.receipt.ReqC:
			in.oa.receipt.Receive(r)
		case r := <-in.oa.staker.ReqC:
			in.oa.staker.Receive(r)
		case r := <-in.oa.validator.ReqC:
			in.oa.validator.Receive(r)
		}
	}
	in.finish(err)

}

func (in *internal) finish(err error) {
	in.oa.controller.Close()
	in.oa.payout.Close()
	in.oa.pipeline.Close()
	in.oa.receipt.Close()
	in.oa.staker.Close()
	in.oa.validator.Close()

	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

// download all of the currently existing validator pipelines
func (in *internal) init(all *sub.ProgramAllResult) error {
	var err error
	in.l_staker, err = in.createLookupStaker()
	if err != nil {
		return err
	}
	in.l_receipt = createLookupReceipt()
	in.l_validator = createLookupValidator()
	in.l_payout = createLookupPayout()
	in.l_pipeline = createLookupPipeline()

	in.unmatchedPeriodRings = make(map[string]cba.PeriodRing)
	in.unmatchedBidLists = make(map[string]cba.BidList)
	in.unmatchedStaker = make(map[string]sub.StakeGroup)
	in.unmatchedReceiptsByPayout = make(map[string]sub.ReceiptGroup)
	in.unmatchedPayouts = make(map[string]sub.PayoutWithData)

	for _, v := range all.BidList {
		in.unmatchedBidLists[v.Data.Pipeline.String()] = v.Data
	}
	for _, v := range all.PeriodRing {
		in.unmatchedPeriodRings[v.Data.Pipeline.String()] = v.Data
	}

	err = all.Stake.Iterate(func(obj *sub.StakeGroup, index uint32, delete func()) error {
		return in.on_stake(*obj)
	})
	if err != nil {
		return err
	}
	err = all.Receipt.Iterate(func(obj *sub.ReceiptGroup, index uint32, delete func()) error {
		return in.on_receipt(*obj)
	})
	if err != nil {
		return err
	}

	err = all.Payout.Iterate(func(obj *sub.PayoutWithData, index uint32, delete func()) error {
		return in.on_payout(*obj)
	})
	if err != nil {
		return err
	}

	// these data items do not reference other data
	err = all.Pipeline.Iterate(func(obj *sub.PipelineGroup, index uint32, delete func()) error {
		return in.on_pipeline(*obj, in.controller.SlotHome())
	})
	if err != nil {
		return err
	}
	err = all.Validator.Iterate(func(obj *sub.ValidatorGroup, index uint32, delete func()) error {
		return in.on_validator(*obj)
	})
	if err != nil {
		return err
	}

	return nil
}
