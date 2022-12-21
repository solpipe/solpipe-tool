package receipt

import (
	"context"

	log "github.com/sirupsen/logrus"
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
	trackHome        *dssub.SubHome[sch.Event]
	pwd              pipe.PayoutWithData
	rwd              rpt.ReceiptWithData
	ps               sch.Schedule
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	rwd rpt.ReceiptWithData,
	trackHome *dssub.SubHome[sch.Event],
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
	in.trackHome = trackHome
	defer trackHome.Close()
	in.ps = ps
	in.pwd = pwd
	in.rwd = rwd

	go loopPeriodClock(in.ctx, in.ps, in.errorC, in.eventC)
	go loopTx(in.ctx, in.rwd, in.errorC, in.eventC)

	var err error
	var send bool
out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case id := <-trackHome.DeleteC:
			trackHome.Delete(id)
		case r := <-trackHome.ReqC:
			trackHome.Receive(r)
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
				in.trackHome.Broadcast(event)
			}

		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
