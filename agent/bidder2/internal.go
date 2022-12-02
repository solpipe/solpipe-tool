package bidder2

import (
	"context"

	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type internal struct {
	ctx    context.Context
	errorC chan<- error
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	router rtr.Router,
) {
	var err error
	errorC := make(chan error, 1)

	in := new(internal)
	in.ctx = ctx
}
