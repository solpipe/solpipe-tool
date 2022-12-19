package pipeline

import (
	"context"

	"github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
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
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.closeSignalCList = make([]chan<- error, 0)
	in.lookAhead = lookAhead
	in.eventHome = eventHome
	in.router = router
	in.pipeline = pipeline
	in.lastPeriodStart = 0
	in.lastPeriodFinish = 0
	in.lastSentAppend = 0
	defer eventHome.Close()

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
out:
	for {
		select {
		case err = <-errorC:
			break out
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
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
			in.eventHome.Broadcast(event)
		case in.lookAhead = <-lookAheadC:
			in.on_latest()
		}
	}
	in.finish(err)
}

func (in *internal) on_latest() {
	if in.lastSentAppend < in.lastPeriodFinish && in.lastPeriodFinish < in.slot+in.lookAhead {
		in.lastSentAppend = in.lastPeriodFinish
		select {
		case <-in.ctx.Done():
		case in.eventC <- sch.Create(
			EVENT_TYPE_READY_APPEND,
			true,
			in.slot,
		):
		}
	}
}

func (in *internal) finish(err error) {
	logrus.Debug(err)
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
				case eventC <- sch.Create(EVENT_TYPE_START, startIsStateChange, slot):
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
			break out
		case eventC <- sch.Create(EVENT_TYPE_FINISH, finishIsStateChange, slot):
		}
	}
}
