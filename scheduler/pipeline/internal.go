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
	cancelAppend     context.CancelFunc
	deleteCancelC    chan<- uint64
	history          *ll.Generic[sch.Event]
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
	deleteCancelC := make(chan uint64)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.deleteCancelC = deleteCancelC
	in.closeSignalCList = make([]chan<- error, 0)
	in.lookAhead = lookAhead
	in.eventHome = eventHome
	defer eventHome.Close()
	in.router = router
	in.pipeline = pipeline
	in.lastPeriodStart = 0
	in.lastPeriodFinish = 0
	in.lastSentAppend = 0
	in.history = ll.CreateGeneric[sch.Event]()

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
			slotSub = in.router.Controller.SlotHome().OnSlot()
		case in.slot = <-slotSub.StreamC:
			in.on_latest()
		case req := <-internalC:
			req(in)
		case id := <-eventHome.DeleteC:
			eventHome.Delete(id)
		case r := <-eventHome.ReqC:
			streamC := eventHome.Receive(r)
			for _, l := range in.history.Array() {
				select {
				case streamC <- l:
				default:
					log.Debug("should not have a stuffed stream channel")
				}
			}
		case err = <-periodSub.ErrorC:
			break out
		case periodRing = <-periodSub.StreamC:
			in.on_period(periodRing)
		case event = <-eventC:
			log.Debugf("pipeline=%s; broadcasting event=%s", event.String())
			in.broadcast(event)
		case in.lookAhead = <-lookAheadC:
			log.Debugf("lookahead=%d from slot=%d", in.lookAhead, in.slot)
			in.on_latest()
		case <-deleteCancelC:
			in.append_cancel()
		}
	}
	in.finish(err)
}

func (in *internal) broadcast(event sch.Event) {
	in.history.Append(event)
	in.eventHome.Broadcast(event)
}

const SLOT_TOO_CLOSE_THRESHOLD uint64 = 50

func (in *internal) append_cancel() {
	log.Debugf("deleting cancel for pipeline")
	if in.cancelAppend != nil {
		in.cancelAppend()
		in.cancelAppend = nil
	}
}

func (in *internal) on_latest() {

	if in.slot == 0 {
		return
	}
	if in.cancelAppend == nil && in.lastPeriodFinish < in.slot+in.lookAhead {

		in.lastSentAppend = in.lastPeriodFinish + 1
		start := in.slot
		if start < in.lastSentAppend {
			start = in.lastSentAppend
		}
		log.Debugf("(pipeline=%s) sending trigger=append_period (%d;%d)", in.pipeline.Id.String(), in.slot, start)
		var trigger *TriggerAppend
		// set start to 0 to make the Solana Program use the actual slot when the transaction is being processed
		if start-in.slot < SLOT_TOO_CLOSE_THRESHOLD {
			start = 0
		}
		trigger, in.cancelAppend = CreateTrigger(in.ctx, in.pipeline, start)
		event := sch.CreateWithPayload(
			sch.TRIGGER_PERIOD_APPEND,
			true,
			in.slot,
			trigger,
		)
		go loopWaitAppend(in.ctx, trigger.Context, in.deleteCancelC, start)
		log.Debugf("sending event=%s", event.String())
		select {
		case <-in.ctx.Done():
		case in.eventC <- event:
		}
	}
}

// this has to exit before another append_period trigger is sent
func loopWaitAppend(
	parentCtx context.Context,
	appendCtx context.Context,
	deleteC chan<- uint64,
	start uint64,
) {
	parentDoneC := parentCtx.Done()
	appendDoneC := appendCtx.Done()
	select {
	case <-parentDoneC:
		return
	case <-appendDoneC:
	}
	select {
	case <-parentDoneC:
	case deleteC <- start:
	}
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *internal) on_period(pr cba.PeriodRing) {

	in.append_cancel()

	n := uint16(len(pr.Ring))
	var k uint16
	var pwd cba.PeriodWithPayout
	for i := uint16(0); i < pr.Length; i++ {
		k = (i + pr.Start) % n
		pwd = pr.Ring[k]

		if !pwd.Period.IsBlank && in.lastPeriodStart < pwd.Period.Start {
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
