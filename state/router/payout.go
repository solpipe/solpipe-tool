package router

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type refPayout struct {
	p    pyt.Payout
	data cba.Payout
}

type lookUpPayout struct {
	byId                map[string]*refPayout
	byPipeline          map[string]map[uint64]*refPayout  // pipeline id -> start -> payout
	bidListWithNoPayout map[string]cba.BidList            // pipeline id  -> bidlist
	receiptWithNoPayout map[string]map[string]rpt.Receipt // payout id -> receipt id -> receipt
}

func createLookupPayout() *lookUpPayout {
	return &lookUpPayout{
		byId:                make(map[string]*refPayout),
		byPipeline:          make(map[string]map[uint64]*refPayout),
		bidListWithNoPayout: make(map[string]cba.BidList),
		receiptWithNoPayout: make(map[string]map[string]rpt.Receipt),
	}
}

func (in *internal) lookup_add_payout(p pyt.Payout, data cba.Payout) {
	ref := &refPayout{p: p, data: data}
	in.l_payout.byId[p.Id.String()] = ref
	{
		x, present := in.l_payout.byPipeline[data.Pipeline.String()]
		if !present {
			x = make(map[uint64]*refPayout)
			in.l_payout.byPipeline[data.Pipeline.String()] = x
		}
		_, present = x[data.Period.Start]
		if !present {
			x[data.Period.Start] = ref
		}
	}
}

func (in *internal) on_payout(pwd sub.PayoutWithData) error {
	id := pwd.Id
	if !pwd.IsOpen {
		y, present := in.l_payout.byId[id.String()]
		if present {
			y.p.Close()
			delete(in.l_payout.byId, id.String())
		}
		return nil
	}
	data := pwd.Data

	var err error
	newlyCreated := false
	ref, present := in.l_payout.byId[id.String()]
	var p pyt.Payout
	if !present {
		p, err = pyt.CreatePayout(in.ctx, pwd)
		in.lookup_add_payout(p, data)
		if err != nil {
			return err
		}
		newlyCreated = true
	} else {
		p = ref.p
	}

	{
		{
			y, present := in.l_payout.bidListWithNoPayout[id.String()]
			if present {
				log.Debugf("fill bid pipeline=%s", id.String())
				p.UpdateBidList(y)
				delete(in.l_payout.bidListWithNoPayout, id.String())
			}
		}
	}
	{
		x, present := in.l_payout.receiptWithNoPayout[id.String()]
		if present {
			for _, r := range x {
				p.UpdateReceipt(r)
			}
			delete(in.l_validator.receiptWithNoValidator, id.String())
		}
	}
	{
		pipeline, present := in.l_pipeline.byId[data.Pipeline.String()]
		if present {
			pipeline.UpdatePayout(p)
		} else {
			y, present := in.l_pipeline.payoutWithNoPipeline[data.Pipeline.String()]
			if !present {
				y = make(map[string]pyt.Payout)
				in.l_pipeline.payoutWithNoPipeline[data.Pipeline.String()] = y
			}
			y[id.String()] = p
		}
	}

	if newlyCreated {
		in.oa.payout.Broadcast(p)
		go loopDelete(in.ctx, p.OnClose(), in.reqClose.payoutCloseC, p.Id, in.ws)
	}
	return nil
}
