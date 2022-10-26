package cranker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx               context.Context
	errorC            chan<- error
	closeSignalCList  []chan<- error
	crankRequestC     chan<- crankRequest
	crankResponseC    <-chan crankResponse
	tryAgainC         chan<- sgo.PublicKey
	config            *relay.Configuration
	pcVaultId         sgo.PublicKey
	pcVault           *sgotkn.Account
	rpc               *sgorpc.Client
	ws                *sgows.Client
	router            rtr.Router
	crankerBalanceSub *sgows.AccountSubscription
	balance           uint64 // balance in SOL
	balanceThreshold  uint64
	slot              uint64
	waitSlot          map[uint64]bool
	status            map[string]*pipelineStatus
	script            *script.Script
}

type pipelineStatus struct {
	pipeline          pipe.Pipeline
	parentData        cba.Pipeline
	lastPeriodStart   uint64 // last period (by start) that has been cranked
	lastAttempedCrank uint64 // last crank we have attempted
	ring              *cba.PeriodRing
	nextCrank         uint64
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	startErrorC chan<- error,
	config *relay.Configuration,
	pcVault *sgotkn.Account,
	balanceThreshold uint64,
	router rtr.Router,
) {
	defer cancel()
	var err error

	errorC := make(chan error, 1)
	crankRequestC := make(chan crankRequest, 1)
	crankResponseC := make(chan crankResponse, 1)
	tryAgainC := make(chan sgo.PublicKey, 1)

	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.crankRequestC = crankRequestC
	in.crankResponseC = crankResponseC
	in.tryAgainC = tryAgainC
	in.config = config
	in.pcVault = pcVault
	in.balance = 0
	in.balanceThreshold = balanceThreshold
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.router = router
	in.balance = 0
	in.slot = 0
	in.status = make(map[string]*pipelineStatus)

	err = in.init(
		crankRequestC,
		crankResponseC,
	)
	startErrorC <- err
	if err != nil {
		return
	}

	doneC := ctx.Done()

	subSlot := in.router.Controller.SlotHome().OnSlot()

	periodSub := router.OnPeriod(util.Zero())
	defer periodSub.Unsubscribe()

	bidSub := router.OnBidSummary()
	defer bidSub.Unsubscribe()

	balC := in.crankerBalanceSub.RecvStream()
	balErrorC := in.crankerBalanceSub.RecvErr()
	defer in.crankerBalanceSub.Unsubscribe()

out:
	for {
		select {
		case id := <-tryAgainC:
			// a previous crank has failed, try to crank again
			status, present := in.status[id.String()]
			if present {
				in.crank(status)
			}
		case resp := <-crankResponseC:
			_, present := in.status[resp.request.pipeline.Id.String()]
			if present {
				if resp.err != nil {
					log.Debug("error in crank")
					os.Stderr.WriteString(resp.err.Error())
					// try again
					//go loopDelayRetry(
					//	ctx,
					//	in.tryAgainC,
					//	resp.request.pipeline.Id,
					//	60*time.Second,
					//	resp.request,
					//)
				}
			}
		case err = <-bidSub.ErrorC:
			break out
		case x := <-bidSub.StreamC:
			in.on_bid(x)
		case err = <-periodSub.ErrorC:
			break out
		case x := <-periodSub.StreamC:
			in.on_period(x)
		case err = <-subSlot.ErrorC:
			break out
		case slot := <-subSlot.StreamC:
			in.on_slot(slot)
		case err = <-balErrorC:
			break out
		case d := <-balC:
			x, ok := d.(*sgows.AccountResult)
			if !ok {
				err = errors.New("bad account result")
				break out
			}
			in.on_balance(x.Value.Lamports)
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	in.finish(err)

}

func loopDelayRetry(
	ctx context.Context,
	tryAgainC chan<- sgo.PublicKey,
	pipelineId sgo.PublicKey,
	delay time.Duration,
	req crankRequest,
) {
	periodSub := req.pipeline.OnPeriod()
	defer periodSub.Unsubscribe()
	select {
	case <-ctx.Done():
	case <-periodSub.ErrorC:
	case <-periodSub.StreamC:
	case <-time.After(delay):
		// we don't have a new period yet, so try again
		select {
		case <-ctx.Done():
		case tryAgainC <- pipelineId:
		}
	}
}

func (in *internal) init(
	crankRequestC <-chan crankRequest,
	crankResponseC chan<- crankResponse,
) error {
	var err error

	in.pcVaultId, err = pcVaultId(in.pcVault)
	if err != nil {
		return err
	}

	for i := 0; i < 1; i++ {

		script, err := in.config.ScriptBuilder(in.ctx)
		if err != nil {
			return err
		}
		go loopCrank(
			in.ctx,
			crankRequestC,
			crankResponseC,
			in.admin(),
			in.pcVaultId,
			in.pcVault.Mint,
			script,
			in.router,
			in.errorC,
		)
	}

	in.rpc = in.config.Rpc()
	in.ws, err = in.config.Ws(in.ctx)
	if err != nil {
		return err
	}

	in.script, err = script.Create(in.ctx, &script.Configuration{Version: in.router.Controller.Version}, in.rpc, in.ws)
	if err != nil {
		return err
	}

	in.slot, err = in.rpc.GetSlot(in.ctx, sgorpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	admin := in.admin()
	in.crankerBalanceSub, err = in.ws.AccountSubscribeWithOpts(
		admin.PublicKey(),
		sgorpc.CommitmentFinalized,
		sgo.EncodingBase64,
	)
	if err != nil {
		return err
	}
	r, err := in.rpc.GetBalance(in.ctx, admin.PublicKey(), sgorpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	in.balance = r.Value

	list, err := in.router.AllPipeline()
	if err != nil {
		return err
	}
	for i := 0; i < len(list); i++ {
		p := list[i]

		log.Debugf("working on pipeline=%s", p.Id.String())
		ring, err := p.PeriodRing()
		if err != nil {
			return err
		}
		in.on_period(ring)
		bidlist, err := p.BidList()
		if err != nil {
			return err
		}
		in.on_bid(*sub.GetBidSummary(&bidlist))
	}

	return nil
}

func (in *internal) finish(err error) {
	log.Debug(err)

	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *internal) admin() sgo.PrivateKey {
	return in.config.Admin
}

func (in *internal) get_status(id sgo.PublicKey) (*pipelineStatus, error) {

	status, present := in.status[id.String()]
	if !present {
		status = new(pipelineStatus)
		pipeline, err := in.router.PipelineById(id)
		if err != nil {
			return nil, err
		}
		status.pipeline = pipeline
		status.parentData, err = pipeline.Data()
		if err != nil {
			return nil, err
		}
		status.lastAttempedCrank = 0
		status.lastPeriodStart = 0
		status.nextCrank = 0
		in.status[id.String()] = status
	}
	return status, nil
}

// upgrade last period crank (by start time)
func (in *internal) on_bid(bs sub.BidSummary) {
	status, err := in.get_status(bs.Pipeline)
	if err != nil {
		log.Debug(err)
		return
	}
	status.lastPeriodStart = bs.LastPeriodStart
}

// on a period, set the next crank time
func (in *internal) on_period(pr cba.PeriodRing) {
	status, err := in.get_status(pr.Pipeline)
	if err != nil {
		log.Debug(err)
		return
	}
	status.ring = &pr

	// check what the next slot time is
	period, present := getNextPeriod(in.slot, pr)
	if !present {
		return
	}
	status.nextCrank = period.Start
}

// find out if we need to run a crank
func (in *internal) on_slot(s uint64) {
	if s%100 == 0 {
		log.Debugf("slot=%d", s)
	}
	in.slot = s
	for _, status := range in.status {
		// for each pipeline, get the period ring?
		if s%100 == 0 {
			log.Debugf("%s___(%d;%d;%d)", status.pipeline.Id.String(), s, status.nextCrank, status.lastAttempedCrank)
		}
		in.crank(status)
	}
}

const RETRY_SLOT = 100

// run the crank in a separate goroutine
func (in *internal) crank(status *pipelineStatus) error {
	if !(status.nextCrank != 0 && status.nextCrank <= in.slot && status.lastAttempedCrank+RETRY_SLOT < in.slot) {
		return nil
	}
	log.Debugf("attempting crank at slot=%d", in.slot)
	{
		list := pipe.ConvertPeriodRingToLinkedList(*status.ring).Array()
		log.Debugf("list size=%d", len(list))
		for _, x := range list {
			log.Debugf("...%d -> %d;  payout=%s", x.Period.Start, x.Period.Start+x.Period.Length, x.Payout.String())
		}
	}
	status.lastAttempedCrank = in.slot
	var err error
	if in.balance <= in.balanceThreshold {
		err = fmt.Errorf("Funds have been depleted.  Current balance: %d", in.balance)
		return err
	}
	// find Period that needs to be cranked
	list := pipe.ConvertPeriodRingToLinkedList(*status.ring).Array()
	var pwd *cba.PeriodWithPayout
out:
	for i := 0; i < len(list); i++ {
		if status.lastPeriodStart < list[i].Period.Start {
			pwd = &list[i]
			break out
		}
	}
	if pwd == nil {
		log.Error("no period available")
		return nil
	}
	req := &crankRequest{
		pipeline:      status.pipeline,
		pipelineData:  status.parentData,
		period:        *pwd,
		attemptedSlot: in.slot,
	}

	//slotSub := in.router.Controller.SlotHome().OnSlot()
	doneC := in.ctx.Done()
	select {
	case <-doneC:
	case in.crankRequestC <- *req:
	}
	/*
		go loopDelaySend(
			in.ctx,
			in.errorC,
			slotSub,
			status.nextCrank,
			in.crankRequestC,
			req,
			in.router,
		)*/

	return nil
}

// send the request once we have reached the time of nextCrank
func loopDelaySend(
	ctx context.Context,
	errorC chan<- error,
	slotSub dssub.Subscription[uint64],
	nextCrank uint64,
	requestC chan<- crankRequest,
	req *crankRequest,
	router rtr.Router,
) {
	defer slotSub.Unsubscribe()
	doneC := ctx.Done()
	slot := uint64(0)
	var err error

out:
	for slot <= nextCrank {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
		}
	}
	if err != nil {
		errorC <- err
		return
	}
	select {
	case <-doneC:
	case requestC <- *req:
	}

}

func (in *internal) on_balance(b uint64) {
	in.balance = b
}

func getNextPeriod(slot uint64, pr cba.PeriodRing) (period cba.Period, present bool) {
	present = false

	for i := uint16(0); i < pr.Length; i++ {
		period = pr.Ring[(pr.Start+i)%uint16(len(pr.Ring))].Period
		// get the current period
		if period.Start+period.Length <= slot {
			// period is late
			present = true
			return
		}
	}

	return
}
