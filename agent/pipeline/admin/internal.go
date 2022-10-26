package admin

import (
	"context"
	"errors"
	"os"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/list"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	"github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	slt "github.com/solpipe/solpipe-tool/state/slot"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx                             context.Context
	errorC                          chan<- error
	deletePayoutC                   chan<- sgo.PublicKey
	closeSignalCList                []chan<- error
	rpc                             *sgorpc.Client
	ws                              *sgows.Client
	rateSettings                    *pba.RateSettings
	periodSettings                  *pba.PeriodSettings
	controller                      ctr.Controller
	router                          rtr.Router
	pipeline                        pipe.Pipeline
	payoutMap                       map[string]pipe.PayoutWithData // payout id -> payout
	nextAttemptToAddPeriod          uint64
	lastAddPeriodAttemptToAddPeriod uint64
	addPeriodResultC                chan<- addPeriodResult
	list                            *ll.Generic[cba.PeriodWithPayout]
	slot                            uint64
	admin                           sgo.PrivateKey
	homeLog                         *sub.SubHome[*pba.LogLine]
	//lastAddPeriod        uint64
}

type addPeriodResult struct {
	err     error
	attempt uint64
}

func DefaultRateSettings() *pba.RateSettings {
	return &pba.RateSettings{
		CrankFee: &pba.Rate{
			Numerator:   0,
			Denominator: 2,
		},
		DecayRate: &pba.Rate{
			Numerator:   1,
			Denominator: 2,
		},
		PayoutShare: &pba.Rate{
			Numerator:   1,
			Denominator: 1,
		},
	}
}

func DefaultPeriodSettings() *pba.PeriodSettings {
	return &pba.PeriodSettings{
		Withhold:  0,
		Lookahead: 3 * 150,
		Length:    150,
	}
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	admin sgo.PrivateKey,
	controller ctr.Controller,
	router rtr.Router,
	pipeline pipe.Pipeline,
	subSlot slt.SlotHome,
	homeLog *sub.SubHome[*pba.LogLine],
	pr *list.Generic[cba.PeriodWithPayout],
	initialSettings *pipe.PipelineSettings,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	addPeriodResultC := make(chan addPeriodResult, 1)
	deletePayoutC := make(chan sgo.PublicKey)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.deletePayoutC = deletePayoutC
	in.rpc = rpcClient
	in.ws = wsClient
	in.closeSignalCList = make([]chan<- error, 0)
	in.periodSettings = DefaultPeriodSettings()
	in.rateSettings = initialSettings.ToProto()
	//in.lastAddPeriod = 0
	in.nextAttemptToAddPeriod = 0
	in.lastAddPeriodAttemptToAddPeriod = 0
	in.addPeriodResultC = addPeriodResultC
	in.list = ll.CreateGeneric[cba.PeriodWithPayout]()
	in.slot = 0
	in.admin = admin
	in.controller = controller
	in.router = router
	in.pipeline = pipeline
	in.payoutMap = make(map[string]pipe.PayoutWithData)
	in.homeLog = homeLog

	slotSubChannelGroup := subSlot.OnSlot()
	defer slotSubChannelGroup.Unsubscribe()
	periodSubChannelGroup := in.pipeline.OnPeriod()
	defer periodSubChannelGroup.Unsubscribe()
	payoutSub := in.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()

	in.on_period(pr)
	// TODO: search for existing payouts
	preaddPayoutC := make(chan pipe.PayoutWithData)
	go loopPreloadPayout(in.ctx, in.pipeline, in.errorC, preaddPayoutC)

	failedAttempts := 0
out:
	for {
		select {
		case id := <-deletePayoutC:
			delete(in.payoutMap, id.String())
		case result := <-addPeriodResultC:
			if result.err != nil {
				os.Stderr.WriteString(result.err.Error())
				//log.Debug(result.err)
				failedAttempts++
				log.Debugf("failed attempts=%d", failedAttempts)
				//log.Debugf("failed to add period: %+v", result.err)
			} else {
				log.Debugf("successfully added period at slot=%d", result.attempt)
				failedAttempts = 0
			}
		case err = <-payoutSub.ErrorC:
			break out
		case x := <-preaddPayoutC:
			// copy the code from "case x := <-payoutSub.StreamC:"
			log.Debugf("reading existing payout=%s", x.Id.String())
			if x.Data.Pipeline.Equals(in.pipeline.Id) {
				_, present := in.payoutMap[x.Payout.Id.String()]
				if !present {
					in.payoutMap[x.Payout.Id.String()] = x
					in.on_payout(x)
					go loopDeletePayout(in.ctx, in.errorC, in.deletePayoutC, x.Payout)
				}
			}
		case x := <-payoutSub.StreamC:
			if x.Data.Pipeline.Equals(in.pipeline.Id) {
				_, present := in.payoutMap[x.Payout.Id.String()]
				if !present {
					in.payoutMap[x.Payout.Id.String()] = x
					in.on_payout(x)
					go loopDeletePayout(in.ctx, in.errorC, in.deletePayoutC, x.Payout)
				}
			}
		case id := <-in.homeLog.DeleteC:
			in.homeLog.Delete(id)
		case x := <-in.homeLog.ReqC:
			in.homeLog.Receive(x)
		case err = <-slotSubChannelGroup.ErrorC:
			log.Debug("slot sub exiting")
			break out
		case slot := <-slotSubChannelGroup.StreamC:
			if slot%50 == 0 {
				log.Debugf("slot=%d", slot)
				//in.log(pba.Severity_INFO, fmt.Sprintf("slot=%d", slot))
			}
			in.on_slot(slot)
		case err = <-periodSubChannelGroup.ErrorC:
			break out
		case pr := <-periodSubChannelGroup.StreamC:
			if pr.Ring == nil {
				pr.Ring = make([]cba.PeriodWithPayout, 0)
			}
			list := ll.CreateGeneric[cba.PeriodWithPayout]()
			for i := uint16(0); i < pr.Length; i++ {
				list.Append(pr.Ring[(i+pr.Start)%uint16(len(pr.Ring))])
			}
			in.on_period(list)
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	if 9 < failedAttempts {
		err = errors.New("too many failed attempts")
	}

	in.finish(err)
}

// look up existing payouts
func loopPreloadPayout(
	ctx context.Context,
	pipeline pipe.Pipeline,
	errorC chan<- error,
	inC chan<- pipe.PayoutWithData,
) {
	doneC := ctx.Done()
	list, err := pipeline.AllPayouts()
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
		return
	}
	for _, pwd := range list {
		select {
		case <-doneC:
			return
		case inC <- pwd:
		}
	}
}

func loopDeletePayout(
	ctx context.Context,
	errorC chan<- error,
	deleteC chan<- sgo.PublicKey,
	payout pyt.Payout,
) {
	var err error
	doneC := ctx.Done()
	id := payout.Id
	closeC := payout.CloseSignal()
	log.Debugf("setting up delete-payout id=%s", id.String())

	select {
	case <-doneC:
	case err = <-closeC:
		if err != nil {
			errorC <- err
		} else {
			log.Debugf("deleting payout=%s", id.String())
			select {
			case <-doneC:
			case deleteC <- id:
			}
		}
	}

}

func (in *internal) log(level pba.Severity, message string) {
	//log.Debugf("sending log=%s", message)
	in.homeLog.Broadcast(&pba.LogLine{Level: level, Message: []string{message}})
}

func (in *internal) finish(err error) {
	if err != nil {
		log.Debug("exiting admin agent loop with err:\n%s", err.Error())
	} else {
		log.Debug("exiting admin agent loop with no error")
	}

	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
