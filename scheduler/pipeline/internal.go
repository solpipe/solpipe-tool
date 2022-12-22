package pipeline

import (
	"context"

	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	eventHome        *dssub.SubHome[sch.Event]
	router           rtr.Router
	pipeline         pipe.Pipeline
	slot             uint64
	eventC           chan<- sch.Event
	lastPeriodStart  uint64
	lastPeriodFinish uint64
	lastSentAppend   uint64
	lookAhead        uint64
	appendCancelList *ll.Generic[*cancelInfo]
	deleteAppendC    chan<- uint64
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	router rtr.Router,
	pipeline pipe.Pipeline,
	internalC <-chan func(*internal),
	eventHome *dssub.SubHome[sch.Event],
	lookAhead uint64,
	lookAheadC <-chan uint64,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 5)
	eventC := make(chan sch.Event, 1)
	deleteAppendC := make(chan uint64)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.deleteAppendC = deleteAppendC
	in.closeSignalCList = make([]chan<- error, 0)
	in.lookAhead = lookAhead
	in.eventHome = eventHome
	defer eventHome.Close()
	in.router = router
	in.pipeline = pipeline
	in.lastPeriodStart = 0
	in.lastPeriodFinish = 0
	in.lastSentAppend = 0
	in.appendCancelList = ll.CreateGeneric[*cancelInfo]()

	periodRing, err := pipeline.PeriodRing()
	if err != nil {
		in.errorC <- err
	}
	in.on_period(periodRing)

	slotSub := in.router.Controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()
	periodSub := in.pipeline.OnPeriod()
	defer periodSub.Unsubscribe()

	var event sch.Event
	log.Debugf("entering loop for scheduler pipeline=%s", pipeline.Id.String())
out:
	for {
		select {
		case err = <-errorC:
			break out
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			//break out
			slotSub = in.router.Controller.SlotHome().OnSlot()
		case in.slot = <-slotSub.StreamC:
			in.on_latest()
		case req := <-internalC:
			req(in)
		case id := <-eventHome.DeleteC:
			eventHome.Delete(id)
		case r := <-eventHome.ReqC:
			eventHome.Receive(r)
		case err = <-periodSub.ErrorC:
			break out
		case periodRing = <-periodSub.StreamC:
			in.on_period(periodRing)
		case event = <-eventC:
			log.Debugf("pipeline=%s; broadcasting event=%s", event.String())
			in.eventHome.Broadcast(event)
		case in.lookAhead = <-lookAheadC:
			log.Debugf("lookahead=%d from slot=%d", in.lookAhead, in.slot)
			in.on_latest()
		case start := <-deleteAppendC:
		found:
			for node := in.appendCancelList.HeadNode(); node != nil; node = node.Next() {
				if node.Value().start == start {
					in.appendCancelList.Remove(node)
					break found
				}
			}

		}
	}
	in.finish(err)
}

type cancelInfo struct {
	cancel context.CancelFunc
	start  uint64
}

func (in *internal) on_latest() {

	for node := in.appendCancelList.HeadNode(); node != nil; node = node.Next() {
		if node.Value().start <= in.lastPeriodFinish {
			node.Value().cancel()
			in.appendCancelList.Remove(node)
		}
	}

	if (in.lastPeriodFinish == 0 || in.lastSentAppend < in.lastPeriodFinish) && in.lastPeriodFinish < in.slot+in.lookAhead {

		in.lastSentAppend = in.lastPeriodFinish
		start := in.slot
		if start < in.lastSentAppend {
			start = in.lastSentAppend
		}

		trigger, cancel := CreateTrigger(in.ctx, in.pipeline, start)
		in.appendCancelList.Append(&cancelInfo{
			cancel: cancel,
			start:  start,
		})
		go loopDeleteAttend(
			in.ctx,
			trigger.Context,
			in.router.Controller,
			in.deleteAppendC,
			start,
		)
		event := sch.CreateWithPayload(
			sch.TRIGGER_PERIOD_APPEND,
			true,
			in.slot,
			trigger,
		)
		log.Debugf("sending event=%s", event.String())
		select {
		case <-in.ctx.Done():
		case in.eventC <- event:
		}
	}
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *internal) on_period(pr cba.PeriodRing) {

	n := uint16(len(pr.Ring))
	var k uint16
	var pwd cba.PeriodWithPayout
	for i := uint16(0); i < pr.Length; i++ {
		k = (i + pr.Start) % n
		pwd = pr.Ring[k]
		if in.lastPeriodStart < pwd.Period.Start {
			in.lastPeriodStart = pwd.Period.Start
			in.lastPeriodFinish = pwd.Period.Start + pwd.Period.Length - 1
			go loopPeriod(
				in.ctx,
				pwd,
				in.router.Controller.SlotHome().OnSlot(),
				in.errorC,
				in.eventC,
			)
			in.on_latest()
		}

	}
}

func loopPeriod(
	ctx context.Context,
	pwd cba.PeriodWithPayout,
	slotSub dssub.Subscription[uint64],
	errorC chan<- error,
	eventC chan<- sch.Event,
) {
	var err error
	doneC := ctx.Done()

	defer slotSub.Unsubscribe()

	start := pwd.Period.Start
	hasStarted := false
	startIsStateChange := false
	finish := start + pwd.Period.Length - 1
	finishIsStateChange := false
	slot := uint64(0)
out:
	for finish <= slot {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			if !hasStarted && start <= slot {
				hasStarted = true
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(
					sch.EVENT_PERIOD_START,
					startIsStateChange,
					slot,
				):
				}
			} else if !startIsStateChange {
				startIsStateChange = true
			}
			if slot < finish {
				finishIsStateChange = true
			}
		}
	}

	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	} else {
		select {
		case <-doneC:
		case eventC <- sch.Create(sch.EVENT_PERIOD_FINISH, finishIsStateChange, slot):
		}
	}
}

func loopDeleteAttend(
	parentCtx context.Context,
	ctx context.Context,
	controller ctr.Controller,
	deleteC chan<- uint64,
	start uint64,
) {

	slotSub := controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()
	parentDoneC := parentCtx.Done()
	doneC := ctx.Done()
	slot := uint64(0)
out:
	for slot < start {
		select {
		case <-parentDoneC:
			return
		case <-doneC:
			break out
		case <-slotSub.ErrorC:
			slotSub = controller.SlotHome().OnSlot()
		case slot = <-slotSub.StreamC:
		}
	}
	select {
	case <-parentDoneC:
	case deleteC <- start:
	}

}
