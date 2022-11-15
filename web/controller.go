package web

import (
	"context"

	cba "github.com/solpipe/cba"
)

func (e1 external) ws_controller(
	clientCtx context.Context,
	errorC chan<- error,
	ansC chan<- cba.Controller,
) {
	doneC := e1.ctx.Done()
	clientDoneC := clientCtx.Done()
	controllerSub := e1.router.Controller.OnData()
	defer controllerSub.Unsubscribe()

	d, err := e1.router.Controller.Data()
	if err == nil {
		ansC <- d
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case <-clientDoneC:
			break out
		case err = <-controllerSub.ErrorC:
			break out
		case d = <-controllerSub.StreamC:
			ansC <- d
		}
	}
	if err != nil {
		errorC <- err
	}
}
