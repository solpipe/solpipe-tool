package pipeline

import (
	"context"
	"errors"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	ctx       context.Context
	cancel    context.CancelFunc
	pipeline  pipe.Pipeline
	eventReqC chan<- dssub.ResponseChannel[sch.Event]
	internalC chan<- func(*internal)
}

// EVENT_TYPE_READY_APPEND
func Schedule(
	ctx context.Context,
	router rtr.Router,
	pipeline pipe.Pipeline,
	initialLookAhead uint64,
	lookAheadC <-chan uint64,
) sch.Schedule {
	ctxC, cancel := context.WithCancel(ctx)
	eventHome := dssub.CreateSubHome[sch.Event]()
	internalC := make(chan func(*internal), 10)
	e1 := external{
		ctx:       ctxC,
		cancel:    cancel,
		pipeline:  pipeline,
		eventReqC: eventHome.ReqC,
	}
	go loopInternal(
		ctxC,
		cancel,
		router,
		pipeline,
		internalC,
		eventHome,
		initialLookAhead,
		lookAheadC,
	)

	return e1
}

func (e1 external) Close() error {
	signalC := e1.CloseSignal()
	e1.cancel()
	return <-signalC
}

func (e1 external) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}

func (e1 external) OnEvent() dssub.Subscription[sch.Event] {
	return dssub.SubscriptionRequest(e1.eventReqC, func(e sch.Event) bool { return true })
}

type TriggerAppend struct {
	Context  context.Context
	Pipeline pipe.Pipeline
	Start    uint64
}

func ReadTrigger(event sch.Event) (*TriggerAppend, error) {
	if event.Payload == nil {
		return nil, errors.New("no trigger payload")
	}
	payload, ok := event.Payload.(*TriggerAppend)
	if !ok {
		return nil, errors.New("bad trigger payload")
	}
	return payload, nil
}
