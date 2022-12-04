package pipeline

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
)

type bidderFeed struct {
	ctx     context.Context
	cancel  context.CancelFunc
	bidC    chan<- BidWithTotal
	submitC chan<- submitInfo
}

type bidderInfo struct {
	list *ll.Generic[*bidderFeed]
	m    map[string]*ll.Node[*bidderFeed]
}

func (in *internal) init_bid() error {
	in.bidderMap = make(map[string]*bidderFeed)
	return nil
}

func (in *internal) on_bid_status(s bidStatusWithStartTime) {
	doneC := in.ctx.Done()
	node, _ := in.periodInfo.find(s.start)
	if node == nil {
		log.Debugf("unable to match start=%d", s.start)
		return
	}
	v := node.Value()

	bi := v.bi
	oldList := bi.list
	newList := ll.CreateGeneric[*bidderFeed]()

	for _, bid := range s.status.Bid {
		var bf *bidderFeed
		sentBidUpdate := false
		oldNode, present := bi.m[bid.User.String()]
		if present {
			bf = oldNode.Value()
			oldList.Remove(oldNode)
			newList.Append(bf)
		} else {
			var present bool
			bf, present = in.bidderMap[bid.User.String()]
			if !present {
				sentBidUpdate = true
				bf = in.bidder_create(
					in.pipelineTpsHome.ReqC,
					in.txFromBidderToValidatorC,
					BidWithTotal{
						Period:               v.Pwd.Data.Period,
						Bid:                  bid,
						TotalDeposit:         s.status.TotalDeposits,
						BandwidthDenominator: s.status.BandwidthDenominator,
					},
				)
				in.bidderMap[bid.User.String()] = bf
			}
		}
		if !sentBidUpdate {
			select {
			case <-doneC:
			case bf.bidC <- BidWithTotal{
				Period:               v.Pwd.Data.Period,
				Bid:                  bid,
				TotalDeposit:         s.status.TotalDeposits,
				BandwidthDenominator: s.status.BandwidthDenominator,
			}:
			}
		}
		bi.m[bid.User.String()] = newList.Append(bf)
	}

}

func (in *internal) bidder_create(
	pipelineTpsReqC chan dssub.ResponseChannel[float64],
	txFromBidderToValidatorC chan<- submitInfo,
	bt BidWithTotal,
) *bidderFeed {
	ctx2, cancel := context.WithCancel(in.ctx)
	bidC := make(chan BidWithTotal, 1)
	txSubmitC := make(chan submitInfo) // no buffer

	go loopBidderInternal(
		ctx2,
		cancel,
		bidC,
		txSubmitC,
		pipelineTpsReqC,
		txFromBidderToValidatorC,
		bt,
	)

	return &bidderFeed{
		ctx:     ctx2,
		cancel:  cancel,
		submitC: txSubmitC,
		bidC:    bidC,
	}
}

type bidderInternal struct {
	ctx           context.Context
	errorC        chan<- error
	allottedShare float64
	pipelineTps   float64
	allotedTps    float64
	txCount       float64
	//	keepReading   bool
	keepReadingC chan<- bool
	boxInterval  time.Duration
	nextBoxC     <-chan time.Time
}

func loopBidderInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	bidC <-chan BidWithTotal,
	txSubmitC <-chan submitInfo,
	pipelineTpsReqC chan dssub.ResponseChannel[float64],
	txFromBidderToValidatorC chan<- submitInfo,
	bt BidWithTotal,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)

	pipelineTpsSub := dssub.SubscriptionRequest(pipelineTpsReqC, func(x float64) bool { return true })
	defer pipelineTpsSub.Unsubscribe()

	bi := new(bidderInternal)
	bi.ctx = ctx
	bi.errorC = errorC

	// bid.BandwidthAllocation
	//bi.allottedShare = float64(bid.BandwidthAllocation) / float64(initPayout.Period.BandwidthAllotment)
	bi.allottedShare = float64(bt.Bid.BandwidthAllocation) / float64(bt.BandwidthDenominator)
	bi.pipelineTps = float64(0)
	bi.allotedTps = float64(0)
	bi.txCount = float64(0)
	bi.boxInterval = 10 * time.Second
	bi.nextBoxC = time.After(bi.boxInterval)

	// rate limit what is coming in via submitC
	loopTxSubmitC := make(chan submitInfo)
	keepReadingC := make(chan bool, 1)
	bi.keepReadingC = keepReadingC
	// implement a "dynamic" select with this goroutine
	go loopBidderSubmitRateLimiter(bi.ctx, txSubmitC, loopTxSubmitC, keepReadingC)

out:
	for {
		select {
		case <-bi.nextBoxC:
			bi.txCount = 0
			bi.keepReadingC <- true
			bi.nextBoxC = time.After(bi.boxInterval)
		case bt = <-bidC:
			bi.allottedShare = float64(bt.Bid.BandwidthAllocation) / float64(bt.BandwidthDenominator)
			bi.allotedTps = bi.pipelineTps * bi.allottedShare
		case s := <-loopTxSubmitC:
			select {
			case <-doneC:
			case txFromBidderToValidatorC <- s:
			}
			bi.update_tps()
		case err = <-pipelineTpsSub.ErrorC:
			break out
		case bi.pipelineTps = <-pipelineTpsSub.StreamC:
			bi.allotedTps = bi.pipelineTps * bi.allottedShare
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		}
	}
	log.Debug(err)
}

// update tps from usage by bidder of capacity
func (bi *bidderInternal) update_tps() {
	bi.txCount += 1
	if bi.allotedTps < (bi.txCount / bi.allotedTps) {
		select {
		case <-bi.ctx.Done():
		case bi.keepReadingC <- false:
		}
	}
}

// This goroutine functions as a kind of dynamic select;
// We want to stop reading submitC when the bidder's TPS exceeds his/her rate limit
// and to restart reading when the bidder's TPS drops back under his/her rate limit
// with the passage of time.
func loopBidderSubmitRateLimiter(
	ctx context.Context,
	inC <-chan submitInfo,
	outC chan<- submitInfo,
	keepReadingC <-chan bool,
) {
	doneC := ctx.Done()
	var si submitInfo
	var keepReading bool
	keepReading = true
out:
	for {
		if keepReading {
			select {
			case <-doneC:
				break out
			case keepReading = <-keepReadingC:
			case si = <-inC:
				// writes to outC will block as the channel has no buffer
				select {
				case <-doneC:
					break out
				case outC <- si:
				}
			}
		} else {
			select {
			case <-doneC:
				break out
			case keepReading = <-keepReadingC:
			}
		}
	}
}

type BidderStatus struct {
	AllotedShare float64
	AllotedTps   float64
	ActualTps    float64
}

type BidWithTotal struct {
	Period               cba.Period
	Bid                  cba.Bid
	TotalDeposit         uint64
	BandwidthDenominator uint64
}

func (bwt BidWithTotal) User() sgo.PublicKey {
	return bwt.Bid.User
}

func (bwt BidWithTotal) AllocatedShare() float64 {
	return float64(bwt.Bid.BandwidthAllocation) / float64(bwt.BandwidthDenominator)
}
