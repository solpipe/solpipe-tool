package pipeline

import (
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type ValidatorUpdate struct {
	Validator val.Validator
	Start     uint64
	Finish    uint64
}

// Get validators and the start/finish time
func (e1 Pipeline) OnValidator() dssub.Subscription[ValidatorUpdate] {
	return dssub.SubscriptionRequest(e1.updateValidatorC, func(x ValidatorUpdate) bool {
		return true
	})
}

func (e1 Pipeline) OnRelativeStake() dssub.Subscription[sub.StakeUpdate] {
	return dssub.SubscriptionRequest(e1.updateStakeStatusC, func(x sub.StakeUpdate) bool {
		return true
	})
}
