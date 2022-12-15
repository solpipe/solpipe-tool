package db

import (
	sgo "github.com/SolmateDev/solana-go"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	"github.com/solpipe/solpipe-tool/util"
)

type Handle interface {
	util.Base
	Initialize(initialState InitialState) error
	PipelineAdd(idList []sgo.PublicKey) error
	PipelineList() ([]sgo.PublicKey, error)
	NetworkAppend(list []NetworkPoint) error
}

type InitialState struct {
	Start  uint64
	Finish uint64
}

type NetworkPoint struct {
	Slot   uint64
	Status ntk.NetworkStatus
}
