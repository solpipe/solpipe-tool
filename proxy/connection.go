package proxy

import (
	"context"
	"errors"

	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func CreateConnectionToPipeline(
	ctx context.Context,
	pipeline pipe.Pipeline,
	t1 *tor.Tor,
) (conn *grpc.ClientConn, err error) {
	if t1 == nil {
		err = errors.New("non-tor connections are not supported")
		return
	}
	log.Debugf("connecting over tor to pipeline=%s", pipeline.Id.String())
	dialer, err := t1.Dialer(ctx, nil)
	if err != nil {
		return nil, err
	}
	return pipeline.Dial(ctx, dialer)
}

func loopCloseConnection(ctx context.Context, conn *grpc.ClientConn) {
	<-ctx.Done()
	conn.Close()
}
