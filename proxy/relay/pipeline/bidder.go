package pipeline

import (
	"context"
	"time"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/slot"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

// used to add a bidder
type bidderInsertInfo struct {
	bid     cba.Bid
	payout  cba.Payout
	periodC chan<- [2]uint64
}

const SLOT_BUFFER_AFTER_PERIOD_ENDS = uint64(10)

// Insert bidders at the start of the payout period.
// Delete bidders once the payout period has finished.
func loopInsertDeleteBidder(
	ctx context.Context,
	errorC chan<- error,
	insertC chan<- bidderInsertInfo,
	deleteC chan<- sgo.PublicKey,
	slotHome slot.SlotHome,
	bid cba.Bid,
	payout cba.Payout,
) {
	doneC := ctx.Done()
	sub := slotHome.OnSlot()
	defer sub.Unsubscribe()

	start := payout.Period.Start
	finish := start + payout.Period.Length
	slot := uint64(0)
	periodC := make(chan [2]uint64, 1)

	hasStarted := false

out:
	for {
		if !hasStarted && start <= slot {
			hasStarted = true
			select {
			case <-doneC:
			case insertC <- bidderInsertInfo{
				bid:     bid,
				payout:  payout,
				periodC: periodC,
			}:
			}
		}
		select {
		case <-doneC:
			break out
		case x := <-periodC:
			// push the finish back within the SLOT_BUFFER to keep the bidder connected
			start = x[0]
			finish = x[1]
		case slot = <-sub.StreamC:
			// add a small safety margin to account for delays in period updates
			if finish+SLOT_BUFFER_AFTER_PERIOD_ENDS < slot {
				select {
				case <-doneC:
				case deleteC <- bid.User:
				}
				break out
			}
		}
	}
}

// update/insert bidder from loopInsertDeleteBidder()
func (in *internal) update_bidders(bi bidderInsertInfo) {
	bf, present := in.bidderMap[bi.bid.User.String()]
	if present {
		select {
		case <-bf.ctx.Done():
		case bf.periodC <- [2]uint64{bi.payout.Period.Start, bi.payout.Period.Start + bi.payout.Period.Length}:
		}
	} else {
		in.bidderMap[bi.bid.User.String()] = in.createBidderFeed(bi.periodC, bi.payout, bi.bid, in.pipelineTpsHome.ReqC, in.txCforBidder)
	}

}

type bidderFeed struct {
	ctx     context.Context
	cancel  context.CancelFunc
	submitC chan<- submitInfo
	periodC chan<- [2]uint64 // used to update period from loopInternal
}

func (in *internal) createBidderFeed(
	periodC chan<- [2]uint64,
	initPayout cba.Payout,
	bid cba.Bid,
	pipelineTpsReqC chan sub.ResponseChannel[float64],
	txFromBidderToValidatorC chan<- submitInfo,
) *bidderFeed {
	ctx2, cancel := context.WithCancel(in.ctx)
	submitC := make(chan submitInfo) // no buffer
	go loopBidderInternal(
		ctx2,
		cancel,
		initPayout,
		bid,
		submitC,
		pipelineTpsReqC,
		txFromBidderToValidatorC,
	)

	return &bidderFeed{
		ctx:     ctx2,
		cancel:  cancel,
		periodC: periodC,
		submitC: submitC,
	}
}

type bidderInternal struct {
	ctx    context.Context
	errorC chan<- error

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
	initPayout cba.Payout,
	bid cba.Bid,
	txSubmitC <-chan submitInfo,
	pipelineTpsReqC chan sub.ResponseChannel[float64],
	txFromBidderToValidatorC chan<- submitInfo,
) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)

	pipelineTpsSub := sub.SubscriptionRequest(pipelineTpsReqC, func(x float64) bool { return true })
	defer pipelineTpsSub.Unsubscribe()

	bi := new(bidderInternal)
	bi.ctx = ctx
	bi.errorC = errorC

	// bid.BandwidthAllocation
	bi.allottedShare = float64(bid.BandwidthAllocation) / float64(initPayout.Period.BandwidthAllotment)
	bi.pipelineTps = float64(0)
	bi.allotedTps = float64(0)
	bi.txCount = float64(0)
	bi.boxInterval = 10 * time.Second
	bi.nextBoxC = time.After(bi.boxInterval)

	// rate limit what is coming in via submitC
	loopTxSubmitC := make(chan submitInfo)
	keepReadingC := make(chan bool, 1)
	bi.keepReadingC = keepReadingC
	go loopBidderSubmitRateLimiter(bi.ctx, txSubmitC, loopTxSubmitC, keepReadingC)

out:
	for {
		select {
		case <-bi.nextBoxC:
			bi.txCount = 0
			bi.keepReadingC <- true
			bi.nextBoxC = time.After(bi.boxInterval)
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

func (bi *bidderInternal) update_tps() {
	bi.txCount += 1
	if bi.allotedTps < (bi.txCount / bi.allotedTps) {
		bi.keepReadingC <- false
	}
}

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
