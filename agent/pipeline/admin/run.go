package admin

import (
	"context"
	"time"

	spt "github.com/solpipe/solpipe-tool/script"
)

func (pi *payoutInternal) run_crank() error {
	if !pi.hasStarted {
		return nil
	}
	if pi.bidIsFinal || pi.bidHasClosed {
		return nil
	}
	var ctxC context.Context
	signalC := make(chan error, 1)
	ctxC, pi.cancelCrank = pi.wrapper.SendDetached(pi.ctx, CRANK_MAX_TRIES, 10*time.Second, func(script *spt.Script) error {
		return runCrank(script, pi.admin, pi.controller, pi.pipeline, pi.payout)
	}, signalC)
	go loopContinueOnError(ctxC, signalC, pi.errorC)

	return nil
}

func (pi *payoutInternal) run_close_bids() error {
	if !pi.bidIsFinal {
		return nil
	}

	if pi.bidHasClosed {
		return nil
	}

	signalC := make(chan error, 1)
	var ctxC context.Context
	ctxC, pi.cancelCloseBid = pi.wrapper.SendDetached(pi.ctx, CLOSE_BIDS_MAX_TRIES, 10*time.Second, func(script *spt.Script) error {
		return runCloseBids(script, pi.admin, pi.controller, pi.pipeline, pi.payout)
	}, signalC)
	go loopContinueOnError(ctxC, signalC, pi.errorC)

	return nil
}

func (pi *payoutInternal) run_close_payout() error {
	if !pi.bidHasClosed {
		return nil
	}
	if !pi.isReadyToClose {
		return nil
	}
	if !pi.validatorAddingIsDone {
		return nil
	}

	// On success a nil will be sent to pi.errorC, thereby closing this payout instance
	// We ignore the cancel() function.
	pi.wrapper.SendDetached(pi.ctx, CLOSE_PAYOUT_MAX_TRIES, 10*time.Second, func(script *spt.Script) error {
		return runClosePayout(script, pi.admin, pi.controller, pi.pipeline, pi.payout)
	}, pi.errorC)

	return nil
}

func loopContinueOnError(
	ctx context.Context,
	resultC <-chan error,
	errorC chan<- error,
) {
	doneC := ctx.Done()
	var err error
	select {
	case <-doneC:
		return
	case err = <-resultC:
	}
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
	}
}
