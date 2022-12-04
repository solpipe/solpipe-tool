package admin

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/list"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	"github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	slt "github.com/solpipe/solpipe-tool/state/slot"
)

type internal struct {
	ctx                             context.Context
	errorC                          chan<- error
	deletePayoutC                   chan<- uint64 // start slot
	closeSignalCList                []chan<- error
	configFilePath                  string
	rpc                             *sgorpc.Client
	ws                              *sgows.Client
	rateSettings                    *pba.RateSettings
	periodSettings                  *pba.PeriodSettings
	controller                      ctr.Controller
	router                          rtr.Router
	pipeline                        pipe.Pipeline
	payoutInfo                      *payoutInfo
	nextAttemptToAddPeriod          uint64
	lastAddPeriodAttemptToAddPeriod uint64
	addPeriodResultC                chan<- addPeriodResult
	list                            *ll.Generic[cba.PeriodWithPayout]
	slot                            uint64
	admin                           sgo.PrivateKey
	homeLog                         *sub.SubHome[*pba.LogLine]
	//lastAddPeriod        uint64
}

type SettingsForFile struct {
	RateSettings   *pba.RateSettings   `json:"rate"`
	PeriodSettings *pba.PeriodSettings `json:"period"`
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
		TickSize:  uint32(script.TICKSIZE_DEFAULT),
		BidSpace:  uint32(script.BIDSPACE_DEFAULT),
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
	configFilePath string,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	addPeriodResultC := make(chan addPeriodResult, 1)
	deletePayoutC := make(chan uint64)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.deletePayoutC = deletePayoutC
	in.configFilePath = configFilePath
	in.rpc = rpcClient
	in.ws = wsClient
	in.closeSignalCList = make([]chan<- error, 0)
	in.periodSettings = DefaultPeriodSettings()
	in.rateSettings = initialSettings.ToProtoRateSettings()
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
	//in.payoutMap = make(map[string]pipe.PayoutWithData)
	in.homeLog = homeLog

	slotSubChannelGroup := subSlot.OnSlot()
	defer slotSubChannelGroup.Unsubscribe()
	payoutSub := in.pipeline.OnPayout()
	defer payoutSub.Unsubscribe()

	in.on_period(pr)
	// TODO: search for existing payouts
	preaddPayoutC := make(chan pipe.PayoutWithData)
	go loopPreloadPayout(in.ctx, in.pipeline, in.errorC, preaddPayoutC)

	failedAttempts := 0

	log.Debug("loading config - 6")

	err = in.init()
	if err != nil {
		in.errorC <- err
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case start := <-deletePayoutC:
			in.payoutInfo.delete(start)
		case result := <-addPeriodResultC:
			if result.err != nil {
				os.Stderr.WriteString(result.err.Error())
				//log.Debug(result.err)
				failedAttempts++
				log.Debugf("failed attempts=%d", failedAttempts)
				in.calculate_next_attempt_to_add_period(false)
				//log.Debugf("failed to add period: %+v", result.err)
			} else {
				log.Debugf("successfully added period at slot=%d", result.attempt)
				failedAttempts = 0
			}
		case err = <-payoutSub.ErrorC:
			break out
		case x := <-payoutSub.StreamC:
			in.on_payout(x)
		case x := <-preaddPayoutC:
			// copy the code from "case x := <-payoutSub.StreamC:"
			log.Debugf("reading existing payout=%s", x.Id.String())
			in.on_payout(x)
		case id := <-in.homeLog.DeleteC:
			in.homeLog.Delete(id)
		case x := <-in.homeLog.ReqC:
			in.homeLog.Receive(x)
		case err = <-slotSubChannelGroup.ErrorC:
			break out
		case slot := <-slotSubChannelGroup.StreamC:
			if slot%50 == 0 {
				log.Debugf("slot=%d", slot)
				//in.log(pba.Severity_INFO, fmt.Sprintf("slot=%d", slot))
			}
			in.on_slot(slot)
		case req := <-internalC:
			req(in)
		}
	}

	if 9 < failedAttempts {
		err = errors.New("too many failed attempts")
	}

	in.finish(err)
}

func (in *internal) init() error {
	var err error
	log.Debug("loading config - 1")
	if in.config_exists() {
		log.Debug("loading config - 2")
		err = in.config_load()
		if err != nil {
			log.Debug("loading config - 3")
			return err
		}
	} else {
		log.Debug("loading config - 4")
		err = in.config_save()
		if err != nil {
			log.Debug("loading config - 5")
			return err
		}
	}
	err = in.init_payout()
	if err != nil {
		return err
	}

	return nil
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

func (in *internal) config_exists() bool {
	log.Debugf("checking config file=%s", in.configFilePath)
	if _, err := os.Stat(in.configFilePath); err == nil {
		return true
	} else {
		return false
	}
}

func (in *internal) config_load() error {
	log.Debugf("loading configuration file from %s", in.configFilePath)
	f, err := os.Open(in.configFilePath)
	if err != nil {
		return err
	}
	c := new(SettingsForFile)
	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		return err
	}

	in.periodSettings = c.PeriodSettings
	in.rateSettings = c.RateSettings
	return nil
}

func (in *internal) config_save() error {
	log.Debugf("saving configuration file to %s", in.configFilePath)
	f, err := os.Open(in.configFilePath)
	if err != nil {
		f, err = os.Create(in.configFilePath)
		if err != nil {
			return err
		}
	}
	c := new(SettingsForFile)
	c.PeriodSettings = in.periodSettings
	c.RateSettings = in.rateSettings
	err = json.NewEncoder(f).Encode(c)
	if err != nil {
		return err
	}
	return nil
}
