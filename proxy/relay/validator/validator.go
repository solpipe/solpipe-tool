package validator

import (
	"context"
	"errors"

	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type external struct {
	ctx       context.Context
	internalC chan<- func(*internal)
	Cancel    context.CancelFunc
	validator val.Validator
	txC       chan<- *submitInfo
}

func Create(
	ctx context.Context,
	validator val.Validator,
	network ntk.Network,
	config relay.Configuration,
) (relay.Relay, error) {

	err := config.Check()
	if err != nil {
		return nil, err
	}
	ctx2, cancel := context.WithCancel(ctx)
	// do not put a buffer here as we want to do rate limiting
	txC := make(chan *submitInfo)
	internalC := make(chan func(*internal), 10)
	go loopInternal(ctx2, cancel, txC, internalC, validator, network, config)
	e1 := external{
		ctx:       ctx,
		internalC: internalC,
		Cancel:    cancel,
		validator: validator,
		txC:       txC,
	}
	return e1, nil
}

func (e1 external) send_cb(ctx context.Context, cb func(in *internal)) error {
	doneC := ctx.Done()
	err := ctx.Err()
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		return errors.New("canceled")
	case e1.internalC <- cb:
		return nil
	}
}
