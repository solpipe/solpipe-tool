package brain

import (
	"context"

	pbb "github.com/solpipe/solpipe-tool/proto/bid"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
)

type Brain struct {
	client pbb.BrainClient
	budget *pbb.TpsBudget
}

type Configuration struct {
	Budget *pbb.TpsBudget `json:"budget"`
}

// work in a single goroutine
func Create(ctx context.Context, conn *grpc.ClientConn, filePath string) (*Brain, error) {

	e1 := &Brain{
		client: pbb.NewBrainClient(conn),
		budget: util.InitBudget(),
	}

	return e1, nil
}
