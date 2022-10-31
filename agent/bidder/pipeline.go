package bidder

import (
	"context"
	"errors"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	"github.com/solpipe/solpipe-tool/proxy"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/sub"
	"google.golang.org/grpc"
)

type periodUpdate struct {
	id   sgo.PublicKey
	ring cba.PeriodRing
}

type bidUpdate struct {
	id   sgo.PublicKey
	list cba.BidList
}

type pipelineInfo struct {
	p              pipe.Pipeline
	pleaseConnectC chan<- struct{}
	periods        []*cba.Period
	bids           []*cba.Bid
	conn           *grpc.ClientConn
	status         *pipelineConnectionStatus
	lastDuration   time.Duration
	allocation     uint16
	ourBid         *cba.Bid
}

func (pi *pipelineInfo) on_period(ring cba.PeriodRing) {
	pi.periods = make([]*cba.Period, ring.Length)
	for i := uint16(0); i < ring.Length; i++ {
		r := ring.Ring[(ring.Start+i)%uint16(len(ring.Ring))]
		pi.periods[i] = &r.Period
	}
}

func (pi *pipelineInfo) on_bid(list cba.BidList) {
	m := ll.CreateGeneric[*cba.Bid]()
	for i := 0; i < len(list.Book); i++ {
		if !list.Book[i].IsBlank {
			z := list.Book[i]
			m.Append(&z)
		}
	}
	pi.bids = m.Array()
}

func (in *internal) on_pipeline(p pipe.Pipeline) {
	log.Debugf("got new pipeline=%s", p.Id.String())
	pleaseConnectC := make(chan struct{}, 1)
	pi := &pipelineInfo{p: p, status: nil, pleaseConnectC: pleaseConnectC, lastDuration: 0}
	in.pipelines[p.Id.String()] = pi
	pleaseConnectC <- struct{}{}
	go loopPipelineConnection(in.ctx, pleaseConnectC, in.connC, p, in.bidder, in.tor)
	go loopBidUpdate(in.ctx, p, in.errorC, in.bidListC)
	go loopPeriodUpdate(in.ctx, p, in.errorC, in.periodRingC)
	go loopUpdateRelativeStake(in.ctx, in.errorC, in.updateChangeActiveStakeC, p)
}

func (in *internal) on_connection_status(x pipelineConnectionStatus) {
	pi, present := in.pipelines[x.id.String()]
	if !present {
		return
	}
	pi.status = &x
	if pi.status.err != nil {
		log.Debug(pi.status.err)
		nextDuration := 5 * time.Second
		if pi.lastDuration != 0 {
			nextDuration = 30 * time.Second
			//nextDuration = pi.lastDuration * 2
		}
		pi.lastDuration = nextDuration
		go loopDelayPleaseConnect(in.ctx, pi.pleaseConnectC, nextDuration)
	}
}

func loopUpdateRelativeStake(
	ctx context.Context,
	errorC chan<- error,
	updateChangeInActiveStateC chan<- sub.StakeUpdate,
	pipeline pipe.Pipeline,
) {
	var err error
	doneC := ctx.Done()
	relativeStakeSub := pipeline.OnRelativeStake()
	defer relativeStakeSub.Unsubscribe()
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-relativeStakeSub.ErrorC:
			break out
		case delta := <-relativeStakeSub.StreamC:
			select {
			case <-doneC:
				break out
			case updateChangeInActiveStateC <- delta:
			}
		}
	}
	if err != nil {
		errorC <- err
	}
}

func loopDelayPleaseConnect(
	ctx context.Context,
	pleaseConnectC chan<- struct{},
	delay time.Duration,
) {
	select {
	case <-ctx.Done():
	case <-time.After(delay):
		pleaseConnectC <- struct{}{}
	}
}

type pipelineConnectionStatus struct {
	id   sgo.PublicKey
	time time.Time
	conn *grpc.ClientConn
	err  error
}

// connecting in a separate goroutine as this takes quite some time
func loopPipelineConnection(
	ctx context.Context,
	pleaseConnectC <-chan struct{},
	statusC chan<- pipelineConnectionStatus,
	p pipe.Pipeline,
	bidderKey sgo.PrivateKey,
	t1 *tor.Tor,
) {
	doneC := ctx.Done()
	var conn *grpc.ClientConn
	var err error
	id := p.Id
	dataSub := p.OnData()
	defer dataSub.Unsubscribe()
	data, err := p.Data()
	if err != nil {
		statusC <- pipelineConnectionStatus{
			id:  p.Id,
			err: err,
		}
		return
	}
	admin := data.Admin

out:
	for {
		select {
		case err = <-dataSub.ErrorC:
			break out
		case x := <-dataSub.StreamC:
			admin = x.Admin
		case <-doneC:
			break out
		case <-pleaseConnectC:
			ctxC, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			conn, err = proxy.CreateConnectionTorClearIfAvailable(
				ctxC,
				admin,
				bidderKey,
				t1,
			)
			if err != nil {
				log.Debugf("failed to connect to pipeline=%s with err: %s", p.Id.String(), err.Error())
			} else {
				log.Debugf("successfully connected to pipeline=%s", p.Id.String())
			}
			statusC <- pipelineConnectionStatus{
				id:   id,
				time: time.Now(),
				err:  err,
				conn: conn,
			}
		}
	}

}

func loopBidUpdate(
	ctx context.Context,
	p pipe.Pipeline,
	errorC chan<- error,
	updateC chan<- bidUpdate,
) {
	doneC := ctx.Done()
	sub := p.OnBid()
	defer sub.Unsubscribe()
	var err error
	var list cba.BidList
	id := p.Id
out:
	for {
		select {
		case <-doneC:
			err = errors.New("canceled")
			break out
		case err = <-sub.ErrorC:
			break out
		case list = <-sub.StreamC:
			updateC <- bidUpdate{
				id: id, list: list,
			}
		}
	}
	select {
	case errorC <- err:
	}
}

func loopPeriodUpdate(
	ctx context.Context,
	p pipe.Pipeline,
	errorC chan<- error,
	updateC chan<- periodUpdate,
) {
	doneC := ctx.Done()
	sub := p.OnPeriod()
	defer sub.Unsubscribe()
	var err error
	var ring cba.PeriodRing
	id := p.Id
out:
	for {
		select {
		case <-doneC:
			err = errors.New("canceled")
			break out
		case err = <-sub.ErrorC:
			break out
		case ring = <-sub.StreamC:
			updateC <- periodUpdate{
				id: id, ring: ring,
			}
		}
	}
	select {
	case errorC <- err:
	}
}
