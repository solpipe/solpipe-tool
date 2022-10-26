package admin

import (
	"context"
	"time"

	pba "github.com/solpipe/solpipe-tool/proto/admin"
	sgo "github.com/SolmateDev/solana-go"
	"google.golang.org/grpc"
)

type Admin struct {
	Client pba.ValidatorClient
}

func Dial(ctx context.Context, url string) (a Admin, err error) {
	ctx2, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	conn, err := grpc.DialContext(ctx2, url, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		return
	}
	go loopClientShutdown(ctx, conn)
	a = Admin{
		Client: pba.NewValidatorClient(conn),
	}
	return
}

func loopClientShutdown(ctx context.Context, conn *grpc.ClientConn) {
	<-ctx.Done()
	conn.Close()
}

func (e1 Admin) SetPipeline(ctx context.Context, pipelineId sgo.PublicKey) error {
	_, err := e1.Client.SetDefault(ctx, &pba.ValidatorSettings{
		PipelineId: pipelineId.String(),
	})
	if err != nil {
		return err
	}
	return nil
}
