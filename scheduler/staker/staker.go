package staker

import (
	"context"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	skr "github.com/solpipe/solpipe-tool/state/staker"
)

type external struct {
	ctx       context.Context
	cancel    context.CancelFunc
	internalC chan<- func(*internal)
	reqC      chan dssub.ResponseChannel[sch.Event]
}

func (e1 external) OnEvent() dssub.Subscription[sch.Event] {
	return dssub.SubscriptionRequest(e1.reqC, func(x sch.Event) bool { return true })
}

// Be alerted as to when to: TRIGGER_STAKER_WITHDRAW
func Schedule(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	vs sch.Schedule,
	s skr.Staker,
) sch.Schedule {
	trackHome := dssub.CreateSubHome[sch.Event]()
	ctxC, cancel := context.WithCancel(ctx)
	internalC := make(chan func(*internal))
	go loopInternal(
		ctxC,
		cancel,
		internalC,
		pwd,
		ps,
		vs,
		s,
		trackHome,
	)

	return external{
		ctx:    ctxC,
		cancel: cancel,
		reqC:   trackHome.ReqC,
	}
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
