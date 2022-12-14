package validator

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	val "github.com/solpipe/solpipe-tool/state/validator"
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

// Be alerted as to when to: TRIGGER_VALIDATOR_SET_PAYOUT, TRIGGER_VALIDATOR_WITHDRAW_RECEIPT
func Schedule(
	ctx context.Context,
	pwd pipe.PayoutWithData,
	pipeline pipe.Pipeline,
	pipelineSchedule sch.Schedule,
	payoutSchedule sch.Schedule,
	v val.Validator,
) sch.Schedule {
	if pwd.Id.String() == "B2weAoPHiLYtANsEPXzsmDrKjoNB1bRs2QZNXat64LiF" {
		log.Debugf("got target")
	}
	trackHome := dssub.CreateSubHome[sch.Event]()
	ctxC, cancel := context.WithCancel(ctx)
	internalC := make(chan func(*internal))
	go loopInternal(
		ctxC,
		cancel,
		internalC,
		pipeline,
		pwd,
		pipelineSchedule,
		payoutSchedule,
		v,
		trackHome,
	)

	return external{
		ctx:    ctxC,
		cancel: cancel,
		reqC:   trackHome.ReqC,
	}
}
func (e1 external) History() ([]sch.Event, error) {
	doneC := e1.ctx.Done()
	ansC := make(chan []sch.Event, 1)
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		ansC <- in.history.Array()
	}:
	}
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case list := <-ansC:
		return list, nil
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
