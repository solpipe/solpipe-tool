package staker

import (
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (e1 Staker) Update(d sub.StakeGroup) {
	e1.dataC <- d
}
