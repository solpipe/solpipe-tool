package admin

import (
	"context"
	"encoding/json"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/ds/sub"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	configFilePath   string
	rateSettings     *pba.RateSettings
	periodSettings   *pba.PeriodSettings
	homeLog          *sub.SubHome[*pba.LogLine]
	periodSettingsC  chan<- *pba.PeriodSettings
	rateSettingsC    chan<- *pba.RateSettings
}

type SettingsForFile struct {
	RateSettings   *pba.RateSettings   `json:"rate"`
	PeriodSettings *pba.PeriodSettings `json:"period"`
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
	homeLog *sub.SubHome[*pba.LogLine],
	initialSettings *pipe.PipelineSettings,
	configFilePath string,
	periodSettingsC chan<- *pba.PeriodSettings,
	rateSettingsC chan<- *pba.RateSettings,
) {
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.configFilePath = configFilePath
	in.closeSignalCList = make([]chan<- error, 0)
	in.periodSettings = DefaultPeriodSettings()
	if initialSettings != nil {
		in.periodSettings.BidSpace = uint32(initialSettings.BidSpace)
	}
	in.periodSettingsC = periodSettingsC
	in.rateSettingsC = rateSettingsC
	log.Debug("settings+++!+!+!+!+")
	log.Debugf("initial settings=%+v", initialSettings)
	in.rateSettings = initialSettings.ToProtoRateSettings()
	//in.lastAddPeriod = 0

	//in.payoutMap = make(map[string]pipe.PayoutWithData)
	in.homeLog = homeLog

	log.Debug("loading config - 6")

	err = in.init()
	if err != nil {
		in.errorC <- err
	}
	in.settings_change()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case id := <-in.homeLog.DeleteC:
			in.homeLog.Delete(id)
		case x := <-in.homeLog.ReqC:
			in.homeLog.Receive(x)
		case req := <-internalC:
			req(in)
		}
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
	f, err := os.OpenFile(in.configFilePath, os.O_WRONLY, 0640)
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

func (in *internal) on_period_settings_update() {
	doneC := in.ctx.Done()
	data, err := json.Marshal(in.periodSettings)
	if err != nil {
		return
	}
	copy := new(pba.PeriodSettings)
	err = json.Unmarshal(data, copy)
	if err != nil {
		return
	}
	select {
	case <-doneC:
	case in.periodSettingsC <- copy:
	}

}

func (in *internal) on_rate_settings_update() {
	doneC := in.ctx.Done()
	data, err := json.Marshal(in.rateSettings)
	if err != nil {
		return
	}
	copy := new(pba.RateSettings)
	err = json.Unmarshal(data, copy)
	if err != nil {
		return
	}
	select {
	case <-doneC:
	case in.rateSettingsC <- copy:
	}
}
