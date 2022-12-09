package script

import (
	"context"
	"errors"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

func Wrap(
	ctx context.Context,
	config *Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
) (Wrapper, error) {
	if config == nil {
		return Wrapper{}, errors.New("no config")
	}

	script := &Script{
		ctx:    ctx,
		rpc:    rpcClient,
		ws:     wsClient,
		config: config,
	}
	internalC := make(chan Callback, 10)

	e1 := Wrapper{
		ctx:       ctx,
		internalC: internalC,
	}
	go loopInternal(ctx, script, internalC)

	return e1, nil
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
