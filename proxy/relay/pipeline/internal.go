package pipeline

import (
	"context"

	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
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

	totalTpsC       chan<- float64
	txCforBidder    chan<- submitInfo // bidders write transactions ( via Submit() ); this channel blocks and has no buffer!
	txCforValidator <-chan submitInfo // validators read.  also rate limiting is done here

	// duplicate internalC to let the validator object manage validatorMap
	validatorInternalC chan<- func(*internal)

	bidderMap    map[string]*bidderFeed    // map userId -> bid
	validatorMap map[string]*validatorFeed // map vote -> validator
}

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	torMgr *tor.Tor,
	dialer *tor.Dialer,
	internalC <-chan func(*internal),
	requestForSubmitChannelC <-chan requestForSubmitChannel,
	slotHome slot.SlotHome,
	network ntk.Network,
	pipeline pipe.Pipeline,
	config relay.Configuration,
) {
	defer cancel()
	var err error

	doneC := ctx.Done()
	errorC := make(chan error, 1)
	pipelineTpsC := make(chan float64, 10)
	txFromBidderToValidatorC := make(chan submitInfo)
	insertValidatorC := make(chan validatorInsertInfo, 1)
	deleteValidatorC := make(chan sgo.PublicKey, 1)
	insertBidderC := make(chan bidderInsertInfo, 1)
	deleteBidderC := make(chan sgo.PublicKey, 1)
	validatorInternalC := make(chan func(*internal), 10)

	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.config = config

	in.tor = torMgr
	in.dialer = dialer
	in.validatorInternalC = validatorInternalC
	in.errorC = errorC
	in.txCforBidder = txFromBidderToValidatorC
	in.txCforValidator = txFromBidderToValidatorC
	in.totalTpsC = pipelineTpsC

	in.slot = 0
	in.network = network
	in.pipeline = pipeline
	in.pipelineTps = 0
	in.pipelineTpsHome = sub.CreateSubHome[float64]()

	in.bidderMap = make(map[string]*bidderFeed)
	in.validatorMap = make(map[string]*validatorFeed)

	// set up subscriptions pulling data from external sources
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	payoutSub := pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	validatorSub := in.pipeline.OnValidator()
	defer validatorSub.Unsubscribe()

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
				req.respC <- bf.submitC
			}

		// handle validators joining and leaving this pipeline
		case err = <-validatorSub.ErrorC:
			break out
		case vu := <-validatorSub.StreamC:
			go loopInsertDeleteValidator(in.ctx, in.errorC, slotHome, vu, insertValidatorC, deleteValidatorC, in.totalTpsC)
		case vii := <-insertValidatorC:
			in.update_validators(vii)
		case id := <-deleteValidatorC:
			vf, present := in.validatorMap[id.String()]
			if present {
				delete(in.validatorMap, id.String())
				vf.cancel()
			}

		// handle new payout (periods) starting and finishing
		// as part of this, track bidders
		case err = <-payoutSub.ErrorC:
			break out
		case p := <-payoutSub.StreamC:
			l, err := pipeline.BidList()
			if err != nil {
				log.Debug("should not have error with bid list")
				break out
			}
			for i := 0; i < len(l.Book); i++ {
				go loopInsertDeleteBidder(
					in.ctx,
					in.errorC,
					insertBidderC,
					deleteBidderC,
					slotHome,
					l.Book[i],
					p.Data,
				)
			}
		case bi := <-insertBidderC:
			in.update_bidders(bi)
		case id := <-deleteBidderC:
			bf, present := in.bidderMap[id.String()]
			if present {
				bf.cancel()
				delete(in.bidderMap, id.String())
			}

		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
