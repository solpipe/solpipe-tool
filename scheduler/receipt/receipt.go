package receipt

import (
	"context"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
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

// Be alerted as to when to: TRIGGER_RECEIPT_APPROVE,EVENT_RECEIPT_APPROVED,EVENT_RECEIPT_NEW_COUNT
func Schedule(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	ps sch.Schedule,
	rwd rpt.ReceiptWithData,
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
		rwd,
		trackHome,
	)

	e1 := external{
		ctx:    ctxC,
		cancel: cancel,
		reqC:   trackHome.ReqC,
	}
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

type TriggerStaker struct {
	Context context.Context
	Staker  skr.Staker
	Receipt rpt.Receipt
	Payout  pyt.Payout
}
