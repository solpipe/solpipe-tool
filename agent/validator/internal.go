package validator

import (
	"context"
	"encoding/json"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type internal struct {
	ctx                context.Context
	errorC             chan<- error
	closeSignalCList   []chan<- error
	config             rly.Configuration
	configFilePath     string
	rpc                *sgorpc.Client
	ws                 *sgows.Client
	scriptWrapper      spt.Wrapper
	controller         ctr.Controller
	router             rtr.Router
	validator          val.Validator
	settings           *pba.ValidatorSettings
	selectedPipeline   *pipelineInfo
	newPayoutC         chan<- payoutWithPipeline
	deletePipelineC    chan<- sgo.PublicKey
	pipelineM          map[string]*pipelineInfo
	payoutM            map[string]*payoutInfo
	eventC             chan<- sch.Event
	latestPeriodFinish uint64
}

const START_BUFFER = uint64(10)

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	serverErrorC <-chan error,
	internalC <-chan func(*internal),
	config rly.Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	scriptWrapper spt.Wrapper,
	router rtr.Router,
	validator val.Validator,
	configFilePath string,
) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()
	eventC := make(chan sch.Event)
	newPayoutC := make(chan payoutWithPipeline)
	deletePipelineC := make(chan sgo.PublicKey)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.eventC = eventC
	in.closeSignalCList = make([]chan<- error, 0)
	in.configFilePath = configFilePath
	in.config = config
	in.rpc = rpcClient
	in.ws = wsClient
	in.scriptWrapper = scriptWrapper
	in.controller = router.Controller
	in.router = router
	in.validator = validator
	in.settings = nil
	in.deletePipelineC = deletePipelineC
	in.newPayoutC = newPayoutC
	in.pipelineM = make(map[string]*pipelineInfo)
	in.payoutM = make(map[string]*payoutInfo)

	if in.config_exists() {
		err = in.config_load()
		if err != nil {
			in.errorC <- err
		}
	}

	log.Debugf("entering loop for validator=%s", in.validator.Id.String())
out:
	for {
		log.Debug("....for loop validator")
		select {
		case err = <-errorC:
			break out
		case err = <-serverErrorC:
			break out
		case <-doneC:
			break out
		case req := <-internalC:
			log.Debugf("validator=%s req start", in.validator.Id.String())
			req(in)
			log.Debugf("validator=%s req finished", in.validator.Id.String())
		case pp := <-newPayoutC:
			log.Debugf("validator on_payout start")
			in.on_payout(pp)
			log.Debugf("validator on_payout finished")
		case event := <-eventC:
			err = in.on_event(event)
			if err != nil {
				break out
			}
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug("exiting agent validator")
	log.Debug(err)
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
	log.Debugf("++++++++++++++++++++++loading configuration file from %s", in.configFilePath)
	f, err := os.Open(in.configFilePath)
	if err != nil {
		return err
	}
	newSettings := new(pba.ValidatorSettings)
	err = json.NewDecoder(f).Decode(newSettings)
	if err != nil {
		return err
	}
	return in.on_settings(newSettings)
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
	err = json.NewEncoder(f).Encode(in.settings)
	if err != nil {
		return err
	}
	return nil
}
