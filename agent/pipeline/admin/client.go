package admin

import (
	"context"
	"time"

	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"google.golang.org/grpc"
)

func Dial(ctx context.Context, url string) (pba.PipelineClient, error) {
	ctx2, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	conn, err := grpc.DialContext(ctx2, url, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	go loopClientShutdown(ctx, conn)
	return pba.NewPipelineClient(conn), nil
}

func loopClientShutdown(ctx context.Context, conn *grpc.ClientConn) {
	<-ctx.Done()
	conn.Close()
}
