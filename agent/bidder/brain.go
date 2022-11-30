package bidder

import (
	"context"
	"errors"
	"io"

	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	pbb "github.com/solpipe/solpipe-tool/proto/bid"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
)

type brainInfo struct {
	budget   *pbb.TpsBudget
	netstats *pbb.NetworkStats
}

func createBrainInfo() *brainInfo {
	return &brainInfo{
		budget: util.InitBudget(),
		netstats: &pbb.NetworkStats{
			NetworkTps: 0,
		},
	}
}

type brainSubGroup struct {
	budget   *dssub.SubHome[*pbb.TpsBudget]
	netstats *dssub.SubHome[*pbb.NetworkStats]
}

type brainReqGroup struct {
	budget   chan<- dssub.ResponseChannel[*pbb.TpsBudget]
	netstats chan<- dssub.ResponseChannel[*pbb.NetworkStats]
}

func createBrainSubGroup() (*brainSubGroup, brainReqGroup) {
	b := &brainSubGroup{
		budget:   dssub.CreateSubHome[*pbb.TpsBudget](),
		netstats: dssub.CreateSubHome[*pbb.NetworkStats](),
	}

	rq := brainReqGroup{
		budget:   b.budget.ReqC,
		netstats: b.netstats.ReqC,
	}
	return b, rq
}

func (bsg *brainSubGroup) Close() {
	bsg.budget.Close()
	bsg.netstats.Close()
}

type external struct {
	pbb.UnimplementedBrainServer
	a Agent
}

func (a Agent) Attach(s *grpc.Server) error {

	e1 := external{a: a}
	pbb.RegisterBrainServer(s, e1)

	return nil
}

func (e1 external) GetTpsBudget(req *pbb.Empty, stream pbb.Brain_GetTpsBudgetServer) (err error) {
	ctx := stream.Context()
	doneC := ctx.Done()
	doneC2 := e1.a.ctx.Done()

	sub := dssub.SubscriptionRequest(e1.a.bsgReq.budget, func(tb *pbb.TpsBudget) bool {
		return true
	})
	defer sub.Unsubscribe()
out:
	for {
		select {
		case <-doneC:
			break out
		case <-doneC2:
			break out
		case err = <-sub.ErrorC:
			break out
		case budget := <-sub.StreamC:
			err = stream.Send(budget)
			if err == io.EOF {
				err = nil
				break out
			} else if err != nil {
				break out
			}
		}
	}
	return
}

func (e1 external) SetTpsBudget(
	ctx context.Context,
	req *pbb.TpsBudget,
) (resp *pbb.TpsBudget, err error) {

	doneC := ctx.Done()
	doneC2 := e1.a.ctx.Done()
	respC := make(chan *pbb.TpsBudget)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case <-doneC2:
		err = errors.New("canceled")
	case e1.a.internalC <- func(in *internal) {
		ans := new(pbb.TpsBudget)
		util.CopyProtoMessage(in.brain.budget, ans)
		respC <- ans
	}:
	}
	if err != nil {
		return
	}
	select {
	case <-doneC:
		err = errors.New("canceled")
	case <-doneC2:
		err = errors.New("canceled")
	case resp = <-respC:
	}
	return
}

func (e1 external) GetStats(req *pbb.Empty, stream pbb.Brain_GetStatsServer) error {
	doneC := e1.a.ctx.Done()
	netSub := dssub.SubscriptionRequest(e1.a.bsgReq.netstats, func(x *pbb.NetworkStats) bool { return true })
	defer netSub.Unsubscribe()

	var err error

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-netSub.ErrorC:
			break out
		case x := <-netSub.StreamC:
			err = stream.Send(&pbb.Stats{
				Data: &pbb.Stats_Network{
					Network: x,
				},
			})
			if err != nil {
				break out
			}
		}
	}

	return err
}
