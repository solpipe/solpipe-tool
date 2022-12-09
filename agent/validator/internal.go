package validator

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	spt "github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
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
	scriptWrapper    spt.Wrapper
	controller       ctr.Controller
	router           rtr.Router
	validator        val.Validator
	settings         *pba.ValidatorSettings
	pipeline         *pipe.Pipeline
	pipelineCtx      context.Context // loopPeriod scoops up new payout accounts
	pipelineCancel   context.CancelFunc
	receiptMap       map[uint64]*validatorReceiptInfo // start->vri
	newPayoutC       chan<- payoutWithPipeline
}

type validatorReceiptInfo struct {
	rsf      *cba.ReceiptWithStartFinish
	ctx      context.Context
	cancel   context.CancelFunc
	rwd      rpt.ReceiptWithData
	pipeline pipe.Pipeline
}

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
	newPayoutC := make(chan payoutWithPipeline)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.configFilePath = configFilePath
	in.config = config
	in.rpc = rpcClient
	in.ws = wsClient
	in.scriptWrapper = scriptWrapper
	//in.slot = 0
	in.controller = router.Controller
	in.router = router
	in.validator = validator
	in.settings = nil
	in.receiptMap = make(map[uint64]*validatorReceiptInfo)
	in.newPayoutC = newPayoutC
	valSub := validator.OnData()
	defer valSub.Unsubscribe()
	dataFetchC := make(chan cba.ValidatorManager)
	go loopFetchValidatorData(in.ctx, validator, in.errorC, dataFetchC)

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
		case err = <-valSub.ErrorC:
			break out
		case x := <-dataFetchC:
			in.on_data(x)
		case x := <-valSub.StreamC:
			in.on_data(x)
		case req := <-internalC:
			req(in)
		case pp := <-newPayoutC:
			in.on_payout(pp)
		}
	}

	in.finish(err)
}

func loopFetchValidatorData(
	ctx context.Context,
	validator val.Validator,
	errorC chan<- error,
	dataC chan<- cba.ValidatorManager,
) {
	doneC := ctx.Done()
	data, err := validator.Data()
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
	} else {
		select {
		case <-doneC:
		case dataC <- data:
		}
	}
}

// The validator Receipt ring tells us if we have a new receipt.
// We do not need to listen to payout.OnReceipt.
func (in *internal) on_data(x cba.ValidatorManager) {
	for _, r := range x.Ring {
		y, present := in.receiptMap[r.Start]
		if !present && !r.HasValidatorWithdrawn {
			rwd, p := in.validator.ReceiptById(r.Receipt)
			if p {
				pwd, err := in.router.PayoutById(rwd.Data.Payout)
				if err != nil {
					in.errorC <- err
					return
				}
				p, err := in.router.PipelineById(pwd.Data.Pipeline)
				if err != nil {
					in.errorC <- err
					return
				}
				ctxC, cancel := context.WithCancel(in.ctx)
				y = &validatorReceiptInfo{
					rsf:      &r,
					ctx:      ctxC,
					cancel:   cancel,
					rwd:      rwd,
					pipeline: p,
				}
				in.receiptMap[r.Start] = y
				in.on_receipt(y)
			} else {
				in.errorC <- errors.New("failed to find receipt=" + r.Receipt.String())
			}

		} else if present && !y.rsf.HasValidatorWithdrawn && r.HasValidatorWithdrawn {
			// false->true means receiptMap needs to be canceled
			y.cancel()
			delete(in.receiptMap, r.Start)
		}
	}
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
