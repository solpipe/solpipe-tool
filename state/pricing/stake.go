package pricing

import (
	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type stakeUpdate struct {
	pipelineId sgo.PublicKey
	relative   sub.StakeUpdate
}
