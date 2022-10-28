package pipeline

import (
	"context"
	"errors"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	pxyclt "github.com/solpipe/solpipe-tool/proxy/client"
	"github.com/solpipe/solpipe-tool/script"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/state/slot"
	val "github.com/solpipe/solpipe-tool/state/validator"
	"google.golang.org/grpc"
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

	vf, present := in.validatorMap[u.data.Vote.String()]
	if present {

		select {
		case <-vf.ctx.Done():
		case vf.periodC <- [2]uint64{u.vu.Start, u.vu.Finish}:
		}
	} else {
		var f *validatorFeed
		var err error
		f, err = in.createValidatorFeed(
			u.vu,
			u.data,
			u.periodC,
			in.totalTpsC,
			in.txCforValidator,
		)
		if err != nil {
			log.Error(err)
		} else {
			in.validatorMap[u.data.Vote.String()] = f
		}

	}

}

type validatorFeed struct {
	ctx       context.Context
	cancel    context.CancelFunc
	data      cba.ValidatorManager
	id        sgo.PublicKey
	connected bool
	periodC   chan<- [2]uint64 // used to update period from loopInternal
}

func (in *internal) createValidatorFeed(
	vu pipe.ValidatorUpdate,
	data cba.ValidatorManager,
	periodC chan<- [2]uint64,
	tpsC chan<- float64,
	txC <-chan submitInfo,
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
		txC,
		in.dialer,
		in.network,
		vu.Validator,
		in.config.Admin,
		scriptBuilder,
		data,
		vu.Start,
		vu.Finish,
	)
	return &validatorFeed{
		ctx:       ctx,
		cancel:    cancel,
		data:      data,
		id:        vu.Validator.Id,
		connected: false,
		periodC:   periodC,
	}, nil
}

type validatorInternal struct {
	ctx                 context.Context
	v                   val.Validator
	dialer              *tor.Dialer
	client              pxyclt.Client
	errorC              chan<- error
	tpsC                chan<- float64
	networkTps          float64
	validatorStakeShare float64
	validatorTps        float64
}

func loopValidatorInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC chan<- func(*internal),
	tpsC chan<- float64,
	txC <-chan submitInfo,
	dialer *tor.Dialer,
	network ntk.Network,
	validator val.Validator,
	admin sgo.PrivateKey,
	scriptBuilder *script.Script,
	data cba.ValidatorManager,
	start uint64,
	finish uint64,
) {
	defer cancel()
	var err error

	doneC := ctx.Done()
	errorC := make(chan error, 1)
	clientC := make(chan pxyclt.Client, 1)
	stakeSub := validator.OnStake()
	defer stakeSub.Unsubscribe()
	networkSub := network.OnNetworkStats()
	defer networkSub.Unsubscribe()

	vi := new(validatorInternal)
	vi.ctx = ctx
	vi.v = validator
	vi.dialer = dialer
	vi.errorC = errorC
	vi.networkTps = 0
	vi.validatorStakeShare = 0
	vi.validatorTps = 0
	vi.tpsC = tpsC

	go loopValidatorConnect(vi.ctx, vi.dialer, clientC, vi.v, admin, scriptBuilder)

	loopTxC := make(chan submitInfo)
	txReadC := make(chan bool, 1)
	go loopValidatorReadTx(vi.ctx, txC, loopTxC, txReadC)

out:
	for {
		select {
		case si := <-loopTxC:
			go loopSendTx(vi.ctx, vi.client, si)
		case err = <-stakeSub.ErrorC:
			break out
		case x := <-stakeSub.StreamC:
			vi.validatorStakeShare = x.Share()
		case err = <-networkSub.ErrorC:
			break out
		case x := <-networkSub.StreamC:
			vi.networkTps = x.AverageTransactionsPerSecond
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case vi.client = <-clientC:
			errorC := make(chan error, 1)
			select {
			case <-doneC:
				break out
			case internalC <- func(in *internal) {
				x, present := in.validatorMap[data.Vote.String()]
				if present {
					x.connected = true
					errorC <- nil
				} else {
					errorC <- errors.New("validator missing!")
				}
			}:
			}
			err = <-errorC
			if err != nil {
				break out
			}
		}
	}

	vi.finish(err)
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
func loopValidatorReadTx(ctx context.Context, inC <-chan submitInfo, outC chan<- submitInfo, keepReadingC <-chan bool) {
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
			case si = <-inC:
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
			case read = <-keepReadingC:
			}
		}
	}
}

func loopValidatorConnect(ctx context.Context, dialer *tor.Dialer, connC chan<- pxyclt.Client, validator val.Validator, admin sgo.PrivateKey, scriptBuidler *script.Script) {
	data, err := validator.Data()
	if err != nil {
		log.Error(err)
		return
	}
	doneC := ctx.Done()
	retryInterval := 60 * time.Second

	nextTryC := time.After(1 * time.Second)
	failures := 0

	var conn *grpc.ClientConn
	var client pxyclt.Client

out:
	for {
		select {
		case <-doneC:
			break out
		case <-nextTryC:
			conn, err = validator.Dial(ctx, dialer)
			if err != nil {
				log.Debug(err)
				err = nil
				nextTryC = time.After(retryInterval)
				failures++
			} else {
				client, err = pxyclt.Create(
					ctx,
					conn,
					nil,
					admin,
					data.Admin,
					nil,
					&script.ReceiptSettings{},
					scriptBuidler,
				)
				if err != nil {
					log.Debug(err)
					conn.Close()
					failures++
					err = nil
					nextTryC = time.After(retryInterval)
				} else {
					connC <- client
					break out
				}
			}
		}
	}
}
