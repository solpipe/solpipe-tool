package script

import (
	"context"
	"errors"
	"time"
)

func Wrap(
	script *Script,
) Wrapper {

	internalC := make(chan Callback, 10)

	e1 := Wrapper{
		ctx:       script.ctx,
		internalC: internalC,
	}
	go loopInternal(script, internalC)

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
	script *Script,
	internalC <-chan Callback,
) {
	ctx := script.ctx
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
	select {
	case <-ctx.Done():
		select {
		case <-time.After(30 * time.Second):
		case errorC <- nil:
		}
	case errorC <- w.Send(ctx, maxTries, delay, cb):
	}

}

func (w Wrapper) Send(
	ctx context.Context,
	maxTries int,
	delay time.Duration,
	cb func(*Script) error,
) error {
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	replay := &CallbackReplay{
		Delay:  delay,
		Count:  0,
		Max:    maxTries,
		ErrorC: errorC,
	}

	select {
	case <-doneC:
	case w.internalC <- func(in *internal) (CallbackReplay, error) {
		replay.Count++
		in.script.ctx = ctx
		return *replay, cb(in.script)
	}:
	}

	select {
	case <-doneC:
		return errors.New("canceled")
	case err := <-errorC:
		return err
	}
}
