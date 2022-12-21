package payout

import (
	"context"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type external struct {
	ctx       context.Context
	cancel    context.CancelFunc
	internalC chan<- func(*internal)
	eventReqC chan<- dssub.ResponseChannel[sch.Event]
}

func Schedule(
	ctx context.Context,
	router rtr.Router,
	pwd pipe.PayoutWithData,
) sch.Schedule {
	ctxC, cancel := context.WithCancel(ctx)
	eventHome := dssub.CreateSubHome[sch.Event]()

	internalC := make(chan func(*internal))
	e1 := external{
		ctx:       ctxC,
		cancel:    cancel,
		internalC: internalC,
		eventReqC: eventHome.ReqC,
	}
	go loopInternal(
		ctxC,
		cancel,
		internalC,
		eventHome,
		router,
		pwd,
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
