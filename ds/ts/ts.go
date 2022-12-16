package db

import (
	sgo "github.com/SolmateDev/solana-go"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/util"
)

type Handle interface {
	util.Base
	Initialize(initialState InitialState) error
	TimeAppend(length uint64) (high uint64, err error)
	PipelineAdd(idList []sgo.PublicKey) error
	PayoutAppend(list []sub.PayoutWithData) error
	BidAppend(bp BidPoint) error
	PipelineList() ([]sgo.PublicKey, error)
	NetworkAppend(list []NetworkPoint) error
	StakeAppend(list []StakePoint) error
}

type InitialState struct {
	Start  uint64
	Finish uint64
}

type NetworkPoint struct {
	Slot   uint64
	Status ntk.NetworkStatus
}

type StakePoint struct {
	Slot       uint64
	PipelineId sgo.PublicKey
	Stake      sub.StakeUpdate
}

type BidPoint struct {
	Slot     uint64
	PayoutId sgo.PublicKey
	Status   pyt.BidStatus
}
