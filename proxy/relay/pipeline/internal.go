package pipeline

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"github.com/solpipe/solpipe-tool/state/slot"
)

type internal struct {
	// manage uptime status
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error

	// tor related
	tor    *tor.Tor
	dialer *tor.Dialer
	config relay.Configuration

	// solana state related
	slot            uint64
	network         ntk.Network
	pipeline        pipe.Pipeline
	pipelineTps     float64               // real time TPS calculation
	pipelineTpsHome *sub.SubHome[float64] // let validators subscribe to pipeline updates

	// relay related
	validatorConnC           chan<- validatorClientWithId
	totalTpsC                chan<- float64
	txFromBidderToValidatorC chan<- submitInfo      // bidders write transactions ( via Submit() ); this channel blocks and has no buffer!
	txCforValidator          <-chan submitInfo      // validators read.  also rate limiting is done here
	validatorInternalC       chan<- func(*internal) // duplicate internalC to let the validator object manage validatorMap
	bx                       *bidListWithSlot
	bidderMap                map[string]*bidderFeed          // map userId -> bid
	validatorConnectionMap   map[string]*validatorConnection // map validator mgr id -> validator connection; connect to all validators
	payoutMap                map[uint64]pipe.PayoutWithData
	deletePayoutC            chan<- uint64

	//validatorMap           map[string]*validatorFeed       // map vote -> validator
}

type bidListWithSlot struct {
	bl *cba.BidList
	s  uint64
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	torMgr *tor.Tor,
	dialer *tor.Dialer,
	internalC <-chan func(*internal),
	requestForSubmitChannelC <-chan requestForSubmitChannel,
	slotHome slot.SlotHome,
	router rtr.Router,
	pipeline pipe.Pipeline,
	config relay.Configuration,
) {
	defer cancel()
	var err error
	network := router.Network
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	pipelineTpsC := make(chan float64, 10)
	txFromBidderToValidatorC := make(chan submitInfo)
	insertValidatorC := make(chan validatorInsertInfo, 1)
	deleteValidatorC := make(chan sgo.PublicKey, 1)

	validatorInternalC := make(chan func(*internal), 10)
	validatorConnC := make(chan validatorClientWithId, 10)
	deletePayoutC := make(chan uint64, 10)

	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.config = config

	in.tor = torMgr
	in.dialer = dialer
	in.validatorConnC = validatorConnC
	in.validatorInternalC = validatorInternalC
	in.errorC = errorC
	in.txFromBidderToValidatorC = txFromBidderToValidatorC
	in.txCforValidator = txFromBidderToValidatorC
	in.totalTpsC = pipelineTpsC
	in.deletePayoutC = deletePayoutC

	in.slot = 0
	in.network = network
	in.pipeline = pipeline
	in.pipelineTps = 0
	in.pipelineTpsHome = sub.CreateSubHome[float64]()

	in.bidderMap = make(map[string]*bidderFeed)
	in.validatorConnectionMap = make(map[string]*validatorConnection)

	in.payoutMap = make(map[uint64]pipe.PayoutWithData)
	in.bx = &bidListWithSlot{
		s: 0,
	}

	// set up subscriptions pulling data from external sources
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	payoutSub := pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	bidSub := pipeline.OnBid()
	defer bidSub.Unsubscribe()
	validatorSub := in.pipeline.OnValidator()
	defer validatorSub.Unsubscribe()

	allValidatorSub := router.OnValidator()
	defer allValidatorSub.Unsubscribe()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case req := <-validatorInternalC:
			req(in)

		// connect to all validators
		case err = <-allValidatorSub.ErrorC:
			break out
		case x := <-allValidatorSub.StreamC:
			valconn, present := in.validatorConnectionMap[x.Data.Vote.String()]
			if !present {
				validator, err2 := router.ValidatorByVote(x.Data.Vote)
				if err2 != nil {
					log.Debug(err2)
				} else {
					script, err := in.config.ScriptBuilder(in.ctx)
					if err != nil {
						break out
					}
					ctxC, cancel := context.WithCancel(in.ctx)
					pleaseConnectC := make(chan struct{}, 1)
					valconn = &validatorConnection{
						v:              validator,
						connected:      false,
						connectionTime: time.Now(), // does not count since client=nil
						pleaseConnectC: pleaseConnectC,
						client:         nil,
						cancel:         cancel,
					}

					in.validatorConnectionMap[x.Id.String()] = valconn

					go loopValidatorConnect(
						ctxC,
						cancel,
						in.tor,
						in.errorC,
						pleaseConnectC,
						in.validatorConnC,
						validator,
						x.Data,
						in.config.Admin,
						script,
					)
				}
			}
			valconn.data = x.Data

		// handle validators joining and leaving this pipeline
		case err = <-validatorSub.ErrorC:
			break out
		case vu := <-validatorSub.StreamC:
			go loopInsertDeleteValidator(in.ctx, in.errorC, slotHome, vu, insertValidatorC, deleteValidatorC, in.totalTpsC)
		case vii := <-insertValidatorC:
			in.update_validators(vii)
		case id := <-deleteValidatorC:
			x, present := in.validatorConnectionMap[id.String()]
			if present {
				if x.feed != nil {
					x.feed.cancel()
					x.feed = nil
				}
			}

		// update connections
		case x := <-validatorConnC:
			y, present := in.validatorConnectionMap[x.id.String()]
			if present {
				y.connectionTime = time.Now()
				y.client = &x.client
			}

		// update the Slot Clock
		case err = <-slotSub.ErrorC:
			break out
		case slot := <-slotSub.StreamC:
			in.slot = slot

		// broadcast TPS updates so we know bidder TPS
		case id := <-in.pipelineTpsHome.DeleteC:
			in.pipelineTpsHome.Delete(id)
		case r := <-in.pipelineTpsHome.ReqC:
			in.pipelineTpsHome.Receive(r)
		case changeInTps := <-pipelineTpsC:
			in.pipelineTps += changeInTps
			in.pipelineTpsHome.Broadcast(in.pipelineTps)

		// send channel of bidder to Submit() function so that we do not burden
		// this select loop with blocking channels
		case req := <-requestForSubmitChannelC:
			bf, present := in.bidderMap[req.sender.String()]
			if present {
				req.bidderFoundC <- true
				req.respC <- bf.submitC
			} else {
				req.bidderFoundC <- false
			}

		case err = <-bidSub.ErrorC:
			break out
		case bl := <-bidSub.StreamC:
			// we only care about final allocations for a given payout period
			// final==true if either period is starting or a period is ending with no subsequent periods
			if bl.AllocationIsFinal {
				log.Debug("have allocation-final flag set to true")
				in.bx.bl = &bl
				in.bx.s = in.slot
				total := uint64(0)
				for _, bid := range in.bx.bl.Book {
					total += bid.BandwidthAllocation
				}
				for _, bid := range in.bx.bl.Book {
					in.bidder_adjust_tps(total, bid)
				}
			}
		// handle new payout (periods) starting and finishing
		// as part of this, track bidders
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			start := pwd.Data.Period.Start
			_, present := in.payoutMap[start]
			if !present {
				in.payoutMap[start] = pwd
				go loopDeletePayout(
					in.ctx,
					in.errorC,
					in.deletePayoutC,
					slotHome,
					start,
					start+pwd.Data.Period.Length,
				)
			}
		case startTime := <-deletePayoutC:
			delete(in.payoutMap, startTime)

		}
	}

	in.finish(err)
}

func loopDeletePayout(
	ctx context.Context,
	errorC chan<- error,
	deleteC chan<- uint64,
	slotHome slot.SlotHome,
	start uint64,
	end uint64,
) {
	doneC := ctx.Done()
	sub := slotHome.OnSlot()
	defer sub.Unsubscribe()
	var err error
	var slot uint64
out:
	for {
		select {
		case <-doneC:
			return
		case err = <-sub.ErrorC:
			break out
		case slot = <-sub.StreamC:
			if end <= slot {
				break out
			}
		}
	}
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
	} else {
		select {
		case <-doneC:
		case deleteC <- start:
		}
	}

}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
