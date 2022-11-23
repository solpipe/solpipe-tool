package brain

import (
	"context"
	"net"
)

type external struct {
}

func Run(ctx context.Context, conn net.Conn) error {

	e1 := external{}

	return e1, nil
}
