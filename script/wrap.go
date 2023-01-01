package script

import (
	"context"
	"errors"
	"time"
)

// ignore the ctx in script
func Wrap(
	ctx context.Context,
	script *Script,
) Wrapper {
	if ctx == nil {
		panic("no context")
	}
	internalC := make(chan func(*internal), 10)
	e1 := Wrapper{
		ctx:       ctx,
		internalC: internalC,
	}
	go loopInternal(ctx, script, internalC)

	return e1
}

type Wrapper struct {
	ctx       context.Context
	internalC chan<- func(*internal)
}

type internal struct {
	ctx    context.Context
	script *Script
}

func loopInternal(
	ctx context.Context,
	script *Script,
	internalC <-chan func(*internal),
) {

	//ctx := script.ctx
	doneC := ctx.Done()
	in := new(internal)
	in.ctx = ctx
	in.script = script

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		}
	}
}

func (w Wrapper) ErrorNonNil(signalC chan<- error) chan<- error {
	errorC := make(chan error, 1)
	go loopErrorNonNill(w.ctx, errorC, signalC)
	return errorC
}

func loopErrorNonNill(
	ctx context.Context,
	aC <-chan error,
	bC chan<- error,
) {
	doneC := ctx.Done()
	var err error
	select {
	case <-doneC:
		return
	case err = <-aC:
	}
	if err != nil {
		select {
		case <-doneC:
		case bC <- err:
		}
	}

}

func (w Wrapper) SendDetached(
	taskCtx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(script *Script) error,
	errorC chan<- error,
) {
	go w.loopSend(taskCtx, maxTries, delay, cb, errorC)
}

func (w Wrapper) loopSend(
	taskCtx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(script *Script) error,
	errorC chan<- error,
) {
	errorC <- w.Send(taskCtx, maxTries, delay, cb)
}

func (w Wrapper) Send(
	taskCtx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(script *Script) error,
) error {
	doneC := w.ctx.Done()
	taskEarlyTerminateC := taskCtx.Done()
	errorC := make(chan error, 1)
	var err error
out:
	for i := 0; i < maxTries; i++ {
		select {
		case <-doneC:
			return errors.New("canceled")
		case <-taskEarlyTerminateC:
			break out
		case w.internalC <- func(in *internal) {
			errorC <- in.run(taskCtx, cb)
		}:
		}
		select {
		case <-doneC:
			return errors.New("canceled")
		case <-taskEarlyTerminateC:
			break out
		case err = <-errorC:
		}
		if err == nil {
			break out
		}
		select {
		case <-doneC:
			return errors.New("canceled")
		case <-taskEarlyTerminateC:
			break out
		case <-time.After(delay):
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (in *internal) run(
	taskCtx context.Context,
	cb func(*Script) error,
) error {
	in.script.txBuilder = nil
	in.script.ctx = taskCtx
	return cb(in.script)
}
