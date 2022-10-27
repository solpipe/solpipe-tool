package staker

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx                  context.Context
	errorC               chan<- error
	dataManager          *sub.StakeGroup
	dataReceiptByReceipt map[string]*sub.StakerReceiptGroup // map receipt id->stakerreceipt
	dataReceiptById      map[string]*sub.StakerReceiptGroup // map stakerreceipt id->stakerreceipt
	stakerHome           *dssub.SubHome[cba.StakerManager]
	cancel               context.CancelFunc
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	data sub.StakeGroup,
	dataStakerManagerC <-chan sub.StakeGroup,
	dataStakerReceiptC <-chan sub.StakerReceiptGroup,
	stakerManagerHome *dssub.SubHome[cba.StakerManager],
	stakerReceiptHome *dssub.SubHome[sub.StakerReceiptGroup],
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	in := new(internal)
	in.ctx = ctx
	in.cancel = cancel
	in.errorC = errorC
	in.dataManager = &data
	in.dataReceiptByReceipt = make(map[string]*sub.StakerReceiptGroup)
	in.dataReceiptById = make(map[string]*sub.StakerReceiptGroup)
	in.stakerHome = stakerManagerHome

	defer stakerManagerHome.Close()
	defer stakerReceiptHome.Close()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case d := <-dataStakerManagerC:
			if d.IsOpen {
				stakerManagerHome.Broadcast(d.Data)
				in.dataManager = &d
			} else {
				log.Debugf("exiting staker; stake member account closed")
				break out
			}
		case d := <-dataStakerReceiptC:
			stakerReceiptHome.Broadcast(d)
			ref := &d
			if d.IsOpen {
				in.dataReceiptByReceipt[d.Data.Receipt.String()] = ref
				in.dataReceiptById[d.Id.String()] = ref
			} else {
				y, present := in.dataReceiptById[d.Id.String()]
				if present {
					delete(in.dataReceiptById, d.Id.String())
					delete(in.dataReceiptByReceipt, y.Data.Receipt.String())
				}
			}
		case id := <-stakerManagerHome.DeleteC:
			stakerManagerHome.Delete(id)
		case d := <-stakerManagerHome.ReqC:
			stakerManagerHome.Receive(d)
		case id := <-stakerReceiptHome.DeleteC:
			stakerReceiptHome.Delete(id)
		case d := <-stakerReceiptHome.ReqC:
			stakerReceiptHome.Receive(d)
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	in.cancel()
	log.Debug(err)
	// closing subscriptions indicate to subscribers that this staker account has been closed
	in.stakerHome.Close()
}
