package receipt

import (
	"context"

	log "github.com/sirupsen/logrus"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	eventC           chan<- sch.Event
	closeSignalCList []chan<- error
	eventHome        *dssub.SubHome[sch.Event]
	pwd              pipe.PayoutWithData
	rwd              rpt.ReceiptWithData
	ps               sch.Schedule
	history          *ll.Generic[sch.Event]
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	rwd rpt.ReceiptWithData,
	eventHome *dssub.SubHome[sch.Event],
) {
	defer cancel()
	eventC := make(chan sch.Event, 1)
	doneC := ctx.Done()
	errorC := make(chan error, 5)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.closeSignalCList = make([]chan<- error, 0)
	in.eventHome = eventHome
	defer eventHome.Close()
	in.ps = ps
	in.pwd = pwd
	in.rwd = rwd
	in.history = ll.CreateGeneric[sch.Event]()

	go loopPeriodClock(in.ctx, in.ps, in.errorC, in.eventC)
	go loopUpdate(in.ctx, in.rwd, in.errorC, in.eventC)

	var err error
	var send bool
out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case id := <-in.eventHome.DeleteC:
			eventHome.Delete(id)
		case r := <-in.eventHome.ReqC:
			streamC := in.eventHome.Receive(r)
			for _, l := range in.history.Array() {
				select {
				case streamC <- l:
				default:
					log.Debug("should not have a stuffed stream channel")
				}
			}
		case event := <-eventC:
			send = true
			switch event.Type {
			case sch.TRIGGER_RECEIPT_APPROVE:
			case sch.EVENT_RECEIPT_APPROVED:
			case sch.EVENT_RECEIPT_NEW_COUNT:
			default:
				send = false
			}
			if send {
				in.broadcast(event)
			}

		}
	}

	in.finish(err)
}

func (in *internal) broadcast(event sch.Event) {
	in.history.Append(event)
	in.eventHome.Broadcast(event)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
