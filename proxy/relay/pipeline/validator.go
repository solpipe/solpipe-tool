package pipeline

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pxyclt "github.com/solpipe/solpipe-tool/proxy/client"
	"github.com/solpipe/solpipe-tool/script"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type validatorInsertInfo struct {
	vu      pipe.ValidatorUpdate
	data    cba.ValidatorManager
	periodC chan<- [2]uint64
}

// Insert the validator once the period starts and delete the validator once the period finishes.
// Take into account that validators may be in subsequent periods.  Allow for updates on the period finish times.
func loopInsertDeleteValidator(
	ctx context.Context,
	errorC chan<- error,
	slotHome slot.SlotHome,
	update pipe.ValidatorUpdate,
	insertC chan<- validatorInsertInfo,
	deleteC chan<- sgo.PublicKey,
	tpsC chan<- float64,
) {
	//finish := update.Finish
	doneC := ctx.Done()
	var err error
	d, err := update.Validator.Data()
	if err != nil {
		log.Error(err)
		return
	}
	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()

	statsSub := update.Validator.OnStats()
	defer statsSub.Unsubscribe()

	start := update.Start
	finish := update.Finish
	periodC := make(chan [2]uint64, 1)

	var slot uint64
	slot = 0
	hasStarted := false
out:
	for {
		if !hasStarted && start <= slot {
			hasStarted = true
			select {
			case <-doneC:
				break out
			case insertC <- validatorInsertInfo{periodC: periodC, data: d, vu: update}:
			}
		}
		select {
		case x := <-periodC:
			start = x[0]
			finish = x[1]
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			// add a small safety margin to account for delays in period updates
			if finish+10 < slot {
				select {
				case <-doneC:
				case deleteC <- d.Vote:
				}
				break out
			}
		}
	}

	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
	}
}

func (in *internal) update_validators(u validatorInsertInfo) {
	doneC := in.ctx.Done()
	valconn, present := in.validatorConnectionMap[u.vu.Validator.Id.String()]
	if !present {
		log.Debugf("validator=%s missing", u.vu.Validator.Id.String())
		return
	}
	vf := valconn.feed

	if vf == nil {
		select {
		case <-doneC:
		case <-vf.ctx.Done():
		case vf.periodC <- [2]uint64{u.vu.Start, u.vu.Finish}:
		}
	} else {
		clientConnC := make(chan pxyclt.Client, 1)
		if valconn.client != nil {
			clientConnC <- *valconn.client
		}

		var err error
		vf, err = in.createValidatorFeed(
			u.vu,
			u.data,
			u.periodC,
			in.totalTpsC,
			in.txCforValidator,
			clientConnC,
			valconn.pleaseConnectC,
		)
		if err != nil {
			log.Error(err)
		} else {
			valconn.feed = vf
		}

	}

}

type validatorConnection struct {
	v              val.Validator
	data           cba.ValidatorManager
	connected      bool
	pleaseConnectC chan<- struct{}
	connectionTime time.Time
	client         *pxyclt.Client
	cancel         context.CancelFunc
	feed           *validatorFeed
}

type validatorFeed struct {
	ctx     context.Context
	cancel  context.CancelFunc
	data    cba.ValidatorManager
	id      sgo.PublicKey
	periodC chan<- [2]uint64 // used to update period from loopInternal
}

func (in *internal) createValidatorFeed(
	vu pipe.ValidatorUpdate,
	data cba.ValidatorManager,
	periodC chan<- [2]uint64,
	tpsC chan<- float64,
	txBidderToValidatorC <-chan submitInfo,
	clientConnC <-chan pxyclt.Client,
	pleaseConnectC chan<- struct{},
) (*validatorFeed, error) {

	ctx, cancel := context.WithCancel(in.ctx)
	scriptBuilder, err := in.config.ScriptBuilder(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	go loopValidatorInternal(
		ctx,
		cancel,
		in.validatorInternalC,
		tpsC,
		txBidderToValidatorC,
		in.network,
		vu.Validator,
		in.config.Admin,
		scriptBuilder,
		data,
		vu.Start,
		vu.Finish,
		clientConnC,
		pleaseConnectC,
	)
	return &validatorFeed{
		ctx:     ctx,
		cancel:  cancel,
		data:    data,
		id:      vu.Validator.Id,
		periodC: periodC,
	}, nil
}

type validatorInternal struct {
	ctx                    context.Context
	v                      val.Validator
	client                 *pxyclt.Client
	errorC                 chan<- error
	tpsC                   chan<- float64
	readOkC                chan<- bool
	networkTps             float64
	validatorStakeShare    float64
	validatorTps           float64
	actualTps              float64
	txCountInCheckInterval float64
	tpsCheckInterval       int64 // seconds
}

func loopValidatorInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC chan<- func(*internal),
	tpsC chan<- float64,
	txSubmitC <-chan submitInfo,
	network ntk.Network,
	validator val.Validator,
	admin sgo.PrivateKey,
	scriptBuilder *script.Script,
	data cba.ValidatorManager,
	start uint64,
	finish uint64,
	clientConnC <-chan pxyclt.Client,
	pleaseConnectC chan<- struct{},
) {
	defer cancel()
	var err error

	doneC := ctx.Done()
	errorC := make(chan error, 1)
	stakeSub := validator.OnStake()
	defer stakeSub.Unsubscribe()
	networkSub := network.OnNetworkStats()
	defer networkSub.Unsubscribe()

	vi := new(validatorInternal)
	vi.ctx = ctx
	vi.v = validator
	vi.errorC = errorC
	vi.networkTps = 0
	vi.validatorStakeShare = 0
	vi.validatorTps = 0
	vi.actualTps = 0
	vi.tpsC = tpsC
	vi.txCountInCheckInterval = 0
	vi.tpsCheckInterval = 3

	//go loopValidatorConnect(vi.ctx, vi.dialer, clientC, vi.v, admin, scriptBuilder)
	txReadyToSendC := make(chan submitInfo)
	txReadSwitchC := make(chan bool, 1)
	vi.readOkC = txReadSwitchC
	go loopValidatorReadTx(vi.ctx, txSubmitC, txReadyToSendC, txReadSwitchC)

out:
	for {
		select {
		case <-time.After(time.Duration(vi.tpsCheckInterval) * time.Second):
			vi.txCountInCheckInterval = 0
			select {
			case <-doneC:
				break out
			case vi.readOkC <- true:
			}
		case si := <-txReadyToSendC:
			go loopSendTx(vi.ctx, *vi.client, si)
			vi.update_actual_tps()
		case client := <-clientConnC:
			vi.client = &client
		case err = <-stakeSub.ErrorC:
			break out
		case x := <-stakeSub.StreamC:
			vi.validatorStakeShare = x.Share()
		case err = <-networkSub.ErrorC:
			break out
		case x := <-networkSub.StreamC:
			vi.networkTps = x.AverageTransactionsPerSecond
			vi.update_tps()
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		}
	}

	vi.finish(err)
}

func (vi *validatorInternal) update_actual_tps() {
	vi.txCountInCheckInterval++
	vi.actualTps = vi.txCountInCheckInterval / float64(vi.tpsCheckInterval)
	if vi.validatorTps <= vi.actualTps {
		select {
		case <-vi.ctx.Done():
		case vi.readOkC <- false:
		}
	}
}

func (vi *validatorInternal) update_tps() {
	oldTps := vi.validatorTps
	newTps := vi.networkTps * vi.validatorStakeShare
	vi.validatorTps = newTps
	select {
	case <-vi.ctx.Done():
	case vi.tpsC <- newTps - oldTps:
	}
}

func (vi *validatorInternal) finish(err error) {
	log.Debug(err)
	// the proxy Client will close via context cancel
}

func loopSendTx(ctx context.Context, client pxyclt.Client, si submitInfo) {
	si.errorC <- client.Submit(ctx, si.tx)
}

// only read from txC when the validator has spare tps capacity.
func loopValidatorReadTx(
	ctx context.Context,
	pendingSendTxC <-chan submitInfo,
	readyToSendTxC chan<- submitInfo,
	keepReadingC <-chan bool,
) {
	read := true
	doneC := ctx.Done()
	var si submitInfo
out:
	for {
		if read {
			select {
			case <-doneC:
				break out
			case read = <-keepReadingC:
			case si = <-pendingSendTxC:
				select {
				case <-doneC:
					break out
				case readyToSendTxC <- si:
				}
			}
		} else {
			select {
			case <-doneC:
				break out
			case read = <-keepReadingC:
			}
		}
	}
}
