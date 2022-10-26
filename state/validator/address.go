package validator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
)

type Dialer interface {
	DialContext(ctx context.Context, network string, addr string) (net.Conn, error)
}

// connect to the validator over Tor
// set dialer to null if there is no proxy to go through
func (e1 Validator) Dial(ctx context.Context, dialer Dialer) (*grpc.ClientConn, error) {
	if dialer == nil {
		return nil, errors.New("no dialer")
	}

	d, err := e1.Data()
	if err != nil {
		return nil, err
	}
	onionID, err := util.GetOnionID(d.Admin.Bytes())
	if err != nil {
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	opts = append(opts, grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
		dialCtx, dialCancel := context.WithTimeout(ctx, d)
		defer dialCancel()
		return dialer.DialContext(dialCtx, "tcp", addr)
	}))
	opts = append(opts, grpc.WithInsecure())

	// the onion address goes here
	return grpc.DialContext(
		ctx,
		fmt.Sprintf("%s.onion:%d", onionID, util.DEFAULT_PROXY_PORT),
		opts...,
	)
}
