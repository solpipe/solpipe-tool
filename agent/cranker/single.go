package cranker

import (
	"context"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	sgo "github.com/SolmateDev/solana-go"
)

type NewPeriodUpdateSub struct {
	PeriodUpdateC <-chan uint64
	DeleteC       chan<- int
	Id            int
}

func (npus NewPeriodUpdateSub) delete() {
	npus.DeleteC <- npus.Id
}

type crankRequest struct {
	pipeline           pipe.Pipeline
	pipelineData       cba.Pipeline
	period             cba.PeriodWithPayout
	attemptedSlot      uint64 // slot
	onSuccessNextCrank uint64
}

type crankResponse struct {
	request crankRequest
	err     error
}

type crankInternal struct {
	ctx       context.Context
	responseC chan<- crankResponse
	admin     sgo.PrivateKey
	pcVault   sgo.PublicKey
	mint      sgo.PublicKey
	script    *script.Script
	router    rtr.Router
}

// Crank a validator pipeline.
func loopCrank(
	ctx context.Context,
	requestC <-chan crankRequest,
	responseC chan<- crankResponse,
	admin sgo.PrivateKey,
	pcVault sgo.PublicKey,
	mint sgo.PublicKey,
	script *script.Script,
	router rtr.Router,
	errorC chan<- error,
) {
	doneC := ctx.Done()
	ci := new(crankInternal)
	ci.ctx = ctx
	ci.responseC = responseC
	ci.admin = admin
	ci.pcVault = pcVault
	ci.mint = mint
	ci.script = script
	ci.router = router
	var err error
out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-requestC:
			err = ci.run(req)
			select {
			case <-doneC:
				err = nil
				break out
			case ci.responseC <- crankResponse{
				request: req,
				err:     err,
			}:
				err = nil
			}

		}
	}
	if err != nil {
		errorC <- err
	}
}

func (ci *crankInternal) run(req crankRequest) error {
	var err error
	err = ci.script.SetTx(ci.admin)
	if err != nil {
		return err
	}
	err = ci.script.Crank(
		ci.router.Controller,
		req.pipeline.Id,
		req.pipelineData.Periods,
		req.pipelineData.Bids,
		req.period.Payout,
		ci.admin,
		ci.pcVault,
	)
	if err != nil {
		return err
	}
	err = ci.script.FinishTx(true)
	if err != nil {
		return err
	}
	return nil
}
