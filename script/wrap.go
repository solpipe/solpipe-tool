package script

import (
	"context"
	"errors"
	"os"
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
	internalC := make(chan Callback, 10)
	e1 := Wrapper{
		ctx:       ctx,
		internalC: internalC,
	}
	go loopInternal(ctx, script, internalC)

	return e1
}

type Wrapper struct {
	ctx       context.Context
	internalC chan<- Callback
}

type internal struct {
	ctx    context.Context
	script *Script
}

type Callback = func(*internal) (CallbackReplay, error)

type CallbackReplay struct {
	Delay  time.Duration
	Count  int
	Max    int
	ErrorC chan<- error
}

func loopInternal(
	ctx context.Context,
	script *Script,
	internalC <-chan Callback,
) {

	//ctx := script.ctx
	doneC := ctx.Done()
	replayC := make(chan Callback, 10)
	in := new(internal)
	in.ctx = ctx
	in.script = script

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-replayC:
			y, err := req(in)
			go loopReplay(in.ctx, y, req, err, replayC)
		case req := <-internalC:
			y, err := req(in)
			if err != nil {
				os.Stderr.WriteString(err.Error())
			}
			go loopReplay(in.ctx, y, req, err, replayC)
		}
	}
}

func loopReplay(
	ctx context.Context,
	cr CallbackReplay,
	cb Callback,
	err error,
	replayC chan<- Callback,
) {
	doneC := ctx.Done()
	if err == nil || cr.Max < cr.Count {
		select {
		case <-doneC:
		case cr.ErrorC <- err:
		}
		return
	}

	select {
	case <-doneC:
		return
	case <-time.After(cr.Delay):
	}

	select {
	case <-doneC:
		return
	case replayC <- cb:
	}

}

func (w Wrapper) SendDetached(
	ctx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(*Script) error,
	signalC chan<- error,
) (context.Context, context.CancelFunc) {
	ctxC, cancel := context.WithCancel(ctx)
	//signalC := make(chan error, 1)
	go w.loop(ctxC, maxTries, delay, cb, signalC)
	return ctxC, cancel
}

func (w Wrapper) loop(
	ctx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(*Script) error,
	errorC chan<- error,
) {
	err := w.Send(ctx, maxTries, delay, cb)
	select {
	case <-ctx.Done():
		select {
		case <-time.After(30 * time.Second):
		case errorC <- nil:
		}
	case errorC <- err:
	}

}

func (w Wrapper) Send(
	ctx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(*Script) error,
) error {
	parentDoneC := w.ctx.Done()
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	replay := &CallbackReplay{
		Delay:  delay,
		Count:  0,
		Max:    maxTries,
		ErrorC: errorC,
	}

	select {
	case <-parentDoneC:
		return errors.New("parent canceled")
	case <-doneC:
	case w.internalC <- func(in *internal) (CallbackReplay, error) {
		replay.Count++
		in.script.ctx = ctx
		cbError := cb(in.script)
		in.script.txBuilder = nil
		return *replay, cbError
	}:
	}

	select {
	case <-parentDoneC:
		return errors.New("parent canceled")
	case <-doneC:
		// handle this error somewhere else
		return nil //errors.New("canceled")
	case err := <-errorC:
		return err
	}
}

func (w Wrapper) ErrorNonNil(
	errorC chan<- error,
) chan<- error {
	signalC := make(chan error, 1)
	go loopPropagate(w.ctx, errorC, signalC)
	return signalC
}

func loopPropagate(
	ctx context.Context,
	errorC chan<- error,
	signalC <-chan error,
) {
	doneC := ctx.Done()
	select {
	case <-doneC:
	case err := <-signalC:
		if err != nil {
			select {
			case <-doneC:
			case errorC <- err:
			}
		}
	}
}
