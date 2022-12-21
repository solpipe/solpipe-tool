package pipeline

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
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
	slotHome         slot.SlotHome

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

	periodInfo             *periodInfo
	validatorConnectionMap map[string]*validatorConnection // map validator mgr id -> validator connection; connect to all validators
	bidderMap              map[string]*bidderFeed          // user_id->bidder
	bidStatusC             chan<- bidStatusWithStartTime
	deletePayoutC          chan<- uint64
	//validatorMap           map[string]*validatorFeed       // map vote -> validator
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
	bidStatusC := make(chan bidStatusWithStartTime)

	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.config = config

	in.slotHome = slotHome
	in.tor = torMgr
	in.dialer = dialer
	in.validatorConnC = validatorConnC
	in.validatorInternalC = validatorInternalC
	in.errorC = errorC
	in.txFromBidderToValidatorC = txFromBidderToValidatorC
	in.txCforValidator = txFromBidderToValidatorC
	in.totalTpsC = pipelineTpsC
	in.deletePayoutC = deletePayoutC
	in.bidStatusC = bidStatusC

	in.slot = 0
	in.network = network
	in.pipeline = pipeline
	in.pipelineTps = 0
	in.pipelineTpsHome = sub.CreateSubHome[float64]()

	in.validatorConnectionMap = make(map[string]*validatorConnection)

	pwdC := make(chan pipe.PayoutWithData)
	go loopPayoutAll(in.ctx, pipeline, pwdC, in.errorC)

	// set up subscriptions pulling data from external sources
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	payoutSub := pipeline.OnPayout()
	defer payoutSub.Unsubscribe()
	validatorSub := in.pipeline.OnValidator()
	defer validatorSub.Unsubscribe()
	allValidatorSub := router.OnValidator()
	defer allValidatorSub.Unsubscribe()

	err = in.init()
	if err != nil {
		errorC <- err
		return
	}

out:
	for {
		select {
		case err = <-errorC:
			break out
		case <-doneC:
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

		case s := <-bidStatusC:
			in.on_bid_status(s)
		// handle new payout (periods) starting and finishing
		// as part of this, track bidders
		case err = <-payoutSub.ErrorC:
			break out
		case pwd := <-payoutSub.StreamC:
			in.on_payout(pwd, slotHome)
		case pwd := <-pwdC:
			in.on_payout(pwd, slotHome)
		case startTime := <-deletePayoutC:
			node, present := in.periodInfo.m[startTime]
			if present {
				log.Debugf("removing payout=%s with start=%d", node.Value().Pwd.Id.String(), node.Value().Pwd.Data.Period.Start)
				in.periodInfo.list.Remove(node)
				node.Value().cancel()
				delete(in.periodInfo.m, startTime)
			}

		}
	}

	in.finish(err)
}

func loopPayoutAll(
	ctx context.Context,
	p pipe.Pipeline,
	outC chan<- pipe.PayoutWithData,
	errorC chan<- error,
) {
	doneC := ctx.Done()
	list, err := p.AllPayouts()
	if err != nil {
		errorC <- err
		return
	}
out:
	for _, pwd := range list {
		select {
		case <-doneC:
			break out
		case outC <- pwd:
		}
	}
}

func (in *internal) init() error {
	err := in.init_period()
	if err != nil {
		return err
	}
	err = in.init_bid()
	if err != nil {
		return err
	}

	return nil
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}
