package validator

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/network"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

func ValidatorMemberId(controllerId sgo.PublicKey, vote sgo.PublicKey) (sgo.PublicKey, uint8, error) {
	// seeds=[b"validator_member",controller.key().as_ref(),vote.key().as_ref()],
	name := "validator_member"
	return sgo.FindProgramAddress([][]byte{
		[]byte(name[:]),
		controllerId.Bytes(),
		vote.Bytes(),
	}, cba.ProgramID)
}

func PayoutId(controllerId sgo.PublicKey, pipelineId sgo.PublicKey, start uint64) (sgo.PublicKey, uint8, error) {
	// seeds=[b"payout",controller.key().as_ref(),pipeline.key().as_ref(),&payout.period.start.to_be_bytes()],
	name := "payout"
	startData := make([]byte, 8)
	binary.BigEndian.PutUint64(startData, start)
	return sgo.FindProgramAddress([][]byte{
		[]byte(name[:]),
		controllerId.Bytes(),
		pipelineId.Bytes(),
		startData,
	}, cba.ProgramID)
}

type Validator struct {
	ctx       context.Context
	cancel    context.CancelFunc
	Id        sgo.PublicKey
	internalC chan<- func(*internal)
	//rpc            *sgorpc.Client
	//ws             *sgows.Client
	updateC        chan<- dssub.ResponseChannel[cba.ValidatorManager]
	stakeRatioC    chan<- dssub.ResponseChannel[StakeStatus]
	updateReceiptC chan<- dssub.ResponseChannel[rpt.ReceiptWithData]
}

type StakeStatus struct {
	Activated uint64
	Total     uint64
}

func ZeroStake() StakeStatus {
	return StakeStatus{Activated: 0, Total: 0}
}

func (s StakeStatus) Share() float64 {
	if s.Total == 0 {
		return 0
	} else {
		return float64(s.Activated) / float64(s.Total)
	}
}

type DeltaStakeStatus struct {
	Old StakeStatus
	New StakeStatus
}

func (ds DeltaStakeStatus) ActiveStakeChange() *big.Int {
	o := big.NewInt(0)
	o.SetUint64(ds.Old.Activated)
	n := big.NewInt(0)
	n.SetUint64(ds.New.Activated)
	ans := big.NewInt(0)
	return ans.Sub(n, o)
}

func DeltaStake(oldStake StakeStatus, newStake StakeStatus) DeltaStakeStatus {
	return DeltaStakeStatus{
		Old: oldStake, New: newStake,
	}
}

func CreateValidator(
	ctx context.Context,
	id sgo.PublicKey,
	data *cba.ValidatorManager,
	rpcClient *sgorpc.Client,
	//wsClient *sgows.Client,
	activatedStakeHome dssub.Subscription[network.VoteStake],
	totalStatkeHome dssub.Subscription[network.VoteStake],
) (Validator, error) {
	var err error
	internalC := make(chan func(*internal), 10)
	validatorHome := dssub.CreateSubHome[cba.ValidatorManager]()
	stakeRatioHome := dssub.CreateSubHome[StakeStatus]()
	receiptHome := dssub.CreateSubHome[rpt.ReceiptWithData]()
	if data == nil {
		err = rpcClient.GetAccountDataBorshInto(ctx, id, data)
		if err != nil {
			return Validator{}, err
		}
	}
	updateC := validatorHome.ReqC
	stakeRatioC := stakeRatioHome.ReqC
	receiptC := receiptHome.ReqC

	ctx2, cancel := context.WithCancel(ctx)

	go loopInternal(
		ctx2,
		cancel,
		internalC,
		id,
		data,
		validatorHome,
		activatedStakeHome,
		totalStatkeHome,
		stakeRatioHome,
		receiptHome,
	)

	return Validator{
		ctx:            ctx2,
		cancel:         cancel,
		Id:             id,
		internalC:      internalC,
		updateC:        updateC,
		stakeRatioC:    stakeRatioC,
		updateReceiptC: receiptC,
	}, nil
}

func (e1 Validator) Close() {
	e1.cancel()
}

func (e1 Validator) OnClose() <-chan struct{} {
	return e1.ctx.Done()
}

// what percent of the network stake does this validator have
func (e1 Validator) StakeRatio() (voteStake uint64, totalStake uint64, err error) {
	err = e1.ctx.Err()
	if err != nil {
		return
	}
	ansC := make(chan uint64, 2)
	e1.internalC <- func(in *internal) {
		ansC <- in.activatedStake
		ansC <- in.totalStake
	}
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case voteStake = <-ansC:
		totalStake = <-ansC
	}

	return
}

func (e1 Validator) Data() (cba.ValidatorManager, error) {
	err := e1.ctx.Err()
	if err != nil {
		return cba.ValidatorManager{}, err
	}
	doneC := e1.ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan cba.ValidatorManager, 1)
	e1.internalC <- func(in *internal) {
		if in.data == nil {
			errorC <- errors.New("no data")
		} else {
			errorC <- nil
			ansC <- *in.data
		}
	}

	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return cba.ValidatorManager{}, err
	} else {
		return <-ansC, nil
	}
}

func (e1 Validator) Print() (string, error) {
	data, err := e1.Data()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Validator:\n\tAdmin=%s\n\tReceipt=%+v\n\tVote=%s", data.Admin.String(), data.Ring, data.Vote.String()), nil
}

func (e1 Validator) OnStats() dssub.Subscription[cba.ValidatorManager] {
	return dssub.SubscriptionRequest(e1.updateC, func(data cba.ValidatorManager) bool {
		return true
	})
}

// get a real time feed of activated state and network state
func (e1 Validator) OnStake() dssub.Subscription[StakeStatus] {
	return dssub.SubscriptionRequest(e1.stakeRatioC, func(data StakeStatus) bool {
		return true
	})
}
