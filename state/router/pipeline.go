package router

import (
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
	"github.com/solpipe/solpipe-tool/state/sub"
	log "github.com/sirupsen/logrus"
)

type lookUpPipeline struct {
	byId                     map[string]pipe.Pipeline
	payoutWithNoPipeline     map[string]map[string]pyt.Payout // pipeline id->payout id->payout
	bidListWithNoPipeline    map[string]cba.BidList           // pipeline id  -> bidlist
	periodRingWithNoPipeline map[string]cba.PeriodRing
}

func createLookupPipeline() *lookUpPipeline {
	return &lookUpPipeline{
		byId:                     make(map[string]pipe.Pipeline),
		payoutWithNoPipeline:     make(map[string]map[string]pyt.Payout),
		bidListWithNoPipeline:    make(map[string]cba.BidList),
		periodRingWithNoPipeline: make(map[string]cba.PeriodRing),
	}
}

func (in *internal) lookup_add_pipeline(p pipe.Pipeline) {
	in.l_pipeline.byId[p.Id.String()] = p
}

func (in *internal) on_pipeline(obj sub.PipelineGroup, slotHome slot.SlotHome) error {

	id := obj.Id
	if !obj.IsOpen {
		y, present := in.l_pipeline.byId[id.String()]
		if present {
			y.Close()
			delete(in.l_pipeline.byId, id.String())
		}
		return nil
	}
	data := obj.Data

	var err error

	p, mainPresent := in.l_pipeline.byId[id.String()]
	if !mainPresent {
		p, err = pipe.CreatePipeline(in.ctx, id, &data, in.rpc, slotHome)
		if err != nil {
			return err
		}
		in.l_pipeline.byId[id.String()] = p
	}

	{
		y, present := in.l_pipeline.bidListWithNoPipeline[id.String()]
		if present {
			log.Debugf("fill bid pipeline=%s", id.String())
			p.UpdateBid(y)
			delete(in.l_pipeline.bidListWithNoPipeline, id.String())
		}

	}
	{
		y, present := in.l_pipeline.periodRingWithNoPipeline[id.String()]
		if present {
			log.Debugf("fill period pipeline=%s", id.String())
			p.UpdatePeriod(y)
			delete(in.l_pipeline.periodRingWithNoPipeline, id.String())
		}

	}
	{
		y, present := in.l_pipeline.payoutWithNoPipeline[id.String()]
		if present {
			for _, payout := range y {
				log.Debugf("fill payout pipeline=%s payout=%s", id.String(), payout.Id.String())
				p.UpdatePayout(payout)
				delete(y, payout.Id.String())
			}
		}

	}

	if !mainPresent {
		// new pipeline, so send a broadcast out
		in.oa.pipeline.Broadcast(p)

		go loopDelete(in.ctx, p.OnClose(), in.reqClose.pipelineCloseC, p.Id, in.ws)
		//in.oa.controllerCloseC
	}

	//in.routerByValidator[data.Validator.String()] = p
	return nil
}

func (in *internal) on_bid(list cba.BidList) {
	p, present := in.l_pipeline.byId[list.Pipeline.String()]
	if !present {
		log.Debugf("pipeline not present (%s)", list.Pipeline.String())
		in.l_pipeline.bidListWithNoPipeline[list.Pipeline.String()] = list
	} else {
		p.UpdateBid(list)
	}
}

func (in *internal) on_period(ring cba.PeriodRing) {

	p, present := in.l_pipeline.byId[ring.Pipeline.String()]
	if !present {
		log.Debugf("pipeline not present (%s)", ring.Pipeline.String())
		in.l_pipeline.periodRingWithNoPipeline[ring.Pipeline.String()] = ring
	} else {
		p.UpdatePeriod(ring)
	}
}