package pipeline

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	schpipe "github.com/solpipe/solpipe-tool/scheduler/pipeline"
	spt "github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type internal struct {
	ctx              context.Context
	closeSignalCList []chan<- error
	errorC           chan<- error
	eventC           chan<- sch.Event
	wrapper          spt.Wrapper
	ps               sch.Schedule
	pipeline         pipe.Pipeline
	router           rtr.Router
	slot             uint64
	periodSettings   *pba.PeriodSettings
	rateSettings     *pba.RateSettings
	admin            sgo.PrivateKey
	payoutM          map[string]*payoutInfo // map: payout id -> *
	deletePayoutC    chan<- sgo.PublicKey
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	router rtr.Router,
	pipeline pipe.Pipeline,
	wrapper spt.Wrapper,
	admin sgo.PrivateKey,
	periodSettingsC <-chan *pba.PeriodSettings,
	rateSettingsC <-chan *pba.RateSettings,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	lookaheadC := make(chan uint64, 1)
	errorC := make(chan error, 5)
	eventC := make(chan sch.Event)
	deletePayoutC := make(chan sgo.PublicKey)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.deletePayoutC = deletePayoutC
	in.closeSignalCList = make([]chan<- error, 0)
	in.router = router
	in.pipeline = pipeline
	in.ps = schpipe.Schedule(
		in.ctx,
		router,
		pipeline,
		0,
		lookaheadC,
	)
	in.admin = admin
	in.payoutM = make(map[string]*payoutInfo)

	pipelineEventSub := in.ps.OnEvent()
	defer pipelineEventSub.Unsubscribe()

	slotSub := router.Controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()

	payoutSub := in.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()

	{
		list, err := in.pipeline.PayoutWithData()
		if err != nil {
			in.errorC <- err
		} else {
			for _, pwd := range list {
				in.on_payout(pwd)
			}
		}
	}

out:
	for {

		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case in.periodSettings = <-periodSettingsC:
			log.Debugf("new period settings=%+v", in.periodSettings)
			select {
			case <-doneC:
				break out
			case lookaheadC <- in.periodSettings.Lookahead:
			}
		case in.rateSettings = <-rateSettingsC:
			log.Debugf("new rate settings=%+v", in.rateSettings)
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			in.on_payout(pwd)
		case err = <-slotSub.ErrorC:
			break out
		case in.slot = <-slotSub.StreamC:
		case err = <-pipelineEventSub.ErrorC:
			break out
		case event := <-pipelineEventSub.StreamC:
			in.on_event(event)
		case event := <-eventC:
			in.on_payout_event(event)
		case id := <-deletePayoutC:
			pi, present := in.payoutM[id.String()]
			if present {
				pi.cancel()
				delete(in.payoutM, id.String())
			}

		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
}
