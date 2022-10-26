package pipeline

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/solpipe/solpipe-tool/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ProxyAddress struct {
	Ssl  bool
	Host string
	Port uint32
}

func ParseProxyAddress(onionAddress string, port uint32) (ans ProxyAddress, err error) {

	ans = ProxyAddress{
		Ssl:  false,
		Host: fmt.Sprintf("%s.onion", onionAddress),
		Port: port,
	}

	return
}

type Dialer interface {
	DialContext(ctx context.Context, network string, addr string) (net.Conn, error)
}

func (pa ProxyAddress) Dial(ctx context.Context, dialer Dialer) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	if dialer != nil {
		opts = append(opts, grpc.WithDialer(func(addr string, d time.Duration) (net.Conn, error) {
			dialCtx, dialCancel := context.WithTimeout(ctx, d)
			defer dialCancel()
			return dialer.DialContext(dialCtx, "tcp", addr)
		}))
	}
	if pa.Ssl {
		config := &tls.Config{
			ServerName: pa.Host,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
		return grpc.DialContext(
			ctx,
			pa.Host,
			opts...,
		)
	} else {
		opts = append(opts, grpc.WithInsecure())
		return grpc.DialContext(
			ctx,
			fmt.Sprintf("%s:%d", pa.Host, pa.Port),
			opts...,
		)
	}
}

// set dialer to null if there is no proxy to go through
func (e1 Pipeline) Dial(ctx context.Context, dialer Dialer) (*grpc.ClientConn, error) {
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
	log.Debugf("from client; pipeline=%s onion id=%s.onion:%d", e1.Id.String(), onionID, util.DEFAULT_PROXY_PORT)

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	opts = append(opts, grpc.WithContextDialer(func(ctxInside context.Context, addr string) (net.Conn, error) {
		return dialer.DialContext(ctxInside, "tcp", addr)
	}))
	opts = append(opts, grpc.WithInsecure())

	// the onion address goes here
	return grpc.DialContext(
		ctx,
		fmt.Sprintf("%s.onion:%d", onionID, util.DEFAULT_PROXY_PORT),
		opts...,
	)
}
