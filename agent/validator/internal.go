package validator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
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
	ctx                   context.Context
	errorC                chan<- error
	closeSignalCList      []chan<- error
	config                rly.Configuration
	configFilePath        string
	rpc                   *sgorpc.Client
	ws                    *sgows.Client
	scriptWrapper         spt.Wrapper
	controller            ctr.Controller
	router                rtr.Router
	validator             val.Validator
	settings              *pba.ValidatorSettings
	pipeline              *pipe.Pipeline
	pipelineCtx           context.Context // loopPeriod scoops up new payout accounts
	pipelineCancel        context.CancelFunc
	receiptMap            map[uint64]*validatorReceiptInfo // start->vri
	receiptAttemptOpen    map[uint64]context.CancelFunc    // start ->cancel()
	deleteReceiptAttemptC chan<- uint64                    // start->cancel()
	newPayoutC            chan<- payoutWithPipeline
	lastStartInPayout     uint64
	slot                  uint64
}

type validatorReceiptInfo struct {
	rsf      *cba.ReceiptWithStartFinish
	ctx      context.Context
	cancel   context.CancelFunc
	rwd      rpt.ReceiptWithData
	pipeline pipe.Pipeline
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
	newPayoutC := make(chan payoutWithPipeline)
	deleteReceiptAttemptC := make(chan uint64)
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
	in.receiptAttemptOpen = make(map[uint64]context.CancelFunc)
	in.newPayoutC = newPayoutC
	in.deleteReceiptAttemptC = deleteReceiptAttemptC
	in.lastStartInPayout = 0
	in.slot = 0
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

	slotSub := in.controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()

	delayInC := make(chan payoutWithPipeline)
	delayOutC := make(chan payoutWithPipeline)
	go loopDelayPayout(in.ctx, delayOutC, delayInC, 10*time.Second)
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
		case err = <-slotSub.ErrorC:
			break out
		case in.slot = <-slotSub.StreamC:
		case pp := <-newPayoutC:
			if in.slot+START_BUFFER < pp.pwd.Data.Period.Start && in.lastStartInPayout < pp.pwd.Data.Period.Start {
				select {
				case <-doneC:
					break out
				case delayOutC <- pp:
					// need to delay calling payout to give a chance for the receipt to pop in
				}
			}

		case pp := <-delayInC:
			in.on_payout(pp)
		case start := <-deleteReceiptAttemptC:
			in.receipt_delete_open_attempt(start)
		}
	}

	in.finish(err)
}

func loopDelayPayout(
	ctx context.Context,
	inC <-chan payoutWithPipeline,
	outC chan<- payoutWithPipeline,
	delay time.Duration,
) {
	list := ll.CreateGeneric[payoutWithPipeline]()
	doneC := ctx.Done()

	var x payoutWithPipeline
	isBlank := true
out:
	for {

		if 0 < list.Size {
			n := time.Now()
			if isBlank {
				x, _ = list.Pop()
				isBlank = false
			}
			diff := int64(0)
			if x.t.After(n) {
				diff = x.t.Unix() - n.Unix()
			}

			select {
			case <-doneC:
				break out
			case <-time.After(time.Duration(diff) * time.Second):
				isBlank = true
				select {
				case <-doneC:
					break out
				case outC <- x:
				}
			case pwp := <-inC:
				list.Append(pwp)
			}
		} else {
			select {
			case <-doneC:
				break out
			case pwp := <-inC:
				list.Append(pwp)
			}
		}

	}

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
		if !present && !r.HasValidatorWithdrawn && in.lastStartInPayout < r.Start {
			rwd, p := in.validator.ReceiptById(r.Receipt)
			if p {
				log.Debugf("evaluating receipt(payout=%s)=%+v", rwd.Data.Payout.String(), rwd)
				pwd, err := in.router.PayoutById(rwd.Data.Payout)
				if err != nil {
					log.Debugf("failed to find payout for receipt=%s", rwd.Data.Payout.String())
					in.errorC <- err
					return
				}
				p, err := in.router.PipelineById(pwd.Data.Pipeline)
				if err != nil {
					log.Debugf("failed to find pipeline for payout=%s", pwd.Payout.Id.String())
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
				in.lastStartInPayout = r.Start
				in.on_receipt(y)
			} else {
				in.errorC <- errors.New("failed to find receipt=" + r.Receipt.String())
			}

		} else if present && r.HasValidatorWithdrawn {
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
