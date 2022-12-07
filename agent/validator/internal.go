package validator

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	config           rly.Configuration
	configFilePath   string
	rpc              *sgorpc.Client
	ws               *sgows.Client
	script           *script.Script
	//slot                uint64
	controller     ctr.Controller
	router         rtr.Router
	validator      val.Validator
	settings       *pba.ValidatorSettings
	pipeline       *pipe.Pipeline
	pipelineCtx    context.Context // loopPeriod scoops up new payout accounts
	pipelineCancel context.CancelFunc
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	serverErrorC <-chan error,
	internalC <-chan func(*internal),
	config rly.Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	script *script.Script,
	router rtr.Router,
	validator val.Validator,
	configFilePath string,
) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.configFilePath = configFilePath
	in.config = config
	in.rpc = rpcClient
	in.ws = wsClient
	in.script = script
	//in.slot = 0
	in.controller = router.Controller
	in.router = router
	in.validator = validator
	in.settings = nil

	if in.config_exists() {
		err = in.config_load()
		if err != nil {
			in.errorC <- err
		}
	}

out:
	for {
		select {
		case err = <-errorC:
			break out
		case err = <-serverErrorC:
			break out
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
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
	return in.settings_change(newSettings)
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

func (a adminExternal) send_cb(ctx context.Context, cb func(in *internal)) error {
	doneC := ctx.Done()
	err := ctx.Err()
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case a.agent.internalC <- cb:
	}
	return err
}
