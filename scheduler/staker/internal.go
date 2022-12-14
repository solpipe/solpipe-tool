package staker

import (
	"context"

	log "github.com/sirupsen/logrus"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	eventC           chan<- sch.Event
	closeSignalCList []chan<- error
	cancelAdd        context.CancelFunc
	cancelWithdraw   context.CancelFunc
	eventHome        *dssub.SubHome[sch.Event]
	pwd              pipe.PayoutWithData
	rwd              rpt.ReceiptWithData
	ps               sch.Schedule
	s                skr.Staker
	history          *ll.Generic[sch.Event]
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	s skr.Staker,
	rwd rpt.ReceiptWithData,
	eventHome *dssub.SubHome[sch.Event],
) {
	defer cancel()
	eventC := make(chan sch.Event, 1)
	doneC := ctx.Done()
	errorC := make(chan error, 5)
	srgtC := make(chan srgWithTransition, 1)
	var srgt srgWithTransition
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
	in.s = s
	in.history = ll.CreateGeneric[sch.Event]()

	var ctxStakerAdd context.Context
	ctxStakerAdd, in.cancelAdd = context.WithCancel(ctx)
	go loopOpenReceipt(ctxStakerAdd, in.cancelAdd, pwd, rwd, s, in.errorC, in.eventC, srgtC)

	var err error

out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case srgt = <-srgtC:
			in.on_staker_receipt(srgt)
		case event := <-eventC:
			in.on_event(event)
		case id := <-in.eventHome.DeleteC:
			in.eventHome.Delete(id)
		case r := <-in.eventHome.ReqC:
			streamC := in.eventHome.Receive(r)
			for _, l := range in.history.Array() {
				select {
				case streamC <- l:
				default:
					log.Debug("should not have a stuffed stream channel")
				}
			}
		}
	}

	in.finish(err)
}

func (in *internal) broadcast(event sch.Event) {
	in.history.Append(event)
	in.eventHome.Broadcast(event)
}

func (in *internal) on_event(event sch.Event) {
	in.broadcast(event)
	switch event.Type {
	case sch.EVENT_STAKER_HAVE_WITHDRAWN:
		if in.cancelWithdraw != nil {
			in.cancelWithdraw()
			in.cancelWithdraw = nil
		}
	default:
	}
}

type srgWithTransition struct {
	rg            sub.StakerReceiptGroup
	isStateChange bool
}

// this function is only called once
func (in *internal) on_staker_receipt(srgt srgWithTransition) {
	// EVENT_STAKER_IS_ADDING
	in.broadcast(sch.Create(
		sch.EVENT_STAKER_IS_ADDING,
		srgt.isStateChange,
		0,
	))
	if in.cancelAdd != nil {
		in.cancelAdd()
		in.cancelAdd = nil
	}
	var ctxC context.Context
	ctxC, in.cancelWithdraw = context.WithCancel(in.ctx)
	go loopWithdraw(
		ctxC,
		in.cancelWithdraw,
		in.ps,
		in.pwd,
		in.rwd,
		in.s,
		srgt.rg,
		in.errorC,
		in.eventC,
	)

}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
