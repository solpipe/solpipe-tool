package validator

import (
	"context"
	"errors"

	pba "github.com/solpipe/solpipe-tool/proto/admin"
	rly "github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	val "github.com/solpipe/solpipe-tool/state/validator"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx                 context.Context
	errorC              chan<- error
	closeSignalCList    []chan<- error
	config              rly.Configuration
	rpc                 *sgorpc.Client
	ws                  *sgows.Client
	script              *script.Script
	slot                uint64
	controller          ctr.Controller
	router              rtr.Router
	validator           val.Validator
	settings            *pba.ValidatorSettings
	pipeline            pipe.Pipeline
	hasPipeline         bool
	payoutScannerCtx    context.Context // loopPeriod scoops up new payout accounts
	payoutScannerCancel context.CancelFunc
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
) {
	defer cancel()
	var err error
	errorC := make(chan error, 1)
	doneC := ctx.Done()
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.config = config
	in.rpc = rpcClient
	in.ws = wsClient
	in.script = script
	in.slot = 0
	in.controller = router.Controller
	in.router = router
	in.validator = validator
	slotSub := in.controller.SlotHome().OnSlot()
	in.settings = &pba.ValidatorSettings{}
	in.hasPipeline = false

out:
	for {
		select {
		case err = <-serverErrorC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case in.slot = <-slotSub.StreamC:
		case <-doneC:
			break out
		case err = <-errorC:
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

type scriptResult struct {
	err    error
	script *script.Script
}

func (in *internal) script_create(ctx context.Context) scriptResult {
	script, err := script.Create(
		in.ctx,
		&script.Configuration{Version: in.config.Version},
		in.config.Rpc(),
		in.ws,
	)
	return scriptResult{err: err, script: script}
}
