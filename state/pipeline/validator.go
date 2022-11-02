package pipeline

import (
	log "github.com/sirupsen/logrus"
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

// Use the activated stake (from staking accounts, not from solpipe account) to calculate real time TPS.
func (e1 Pipeline) OnRelativeStake() dssub.Subscription[sub.StakeUpdate] {
	return dssub.SubscriptionRequest(e1.updateStakeStatusC, func(x sub.StakeUpdate) bool {
		return true
	})
}

func (e1 Pipeline) loopUpdateValidatorByVote(v val.Validator, start uint64, finish uint64) {
	log.Debugf("looping ")
	d, err := v.Data()
	if err != nil {
		return
	}
	slotSub := e1.slotSub.OnSlot()
	defer slotSub.Unsubscribe()
	vote := d.Vote
	sub := v.OnStake()
	defer sub.Unsubscribe()
	presentC := make(chan bool, 1)
	errorC2 := make(chan chan<- error, 1)
	e1.internalC <- func(in *internal) {
		in.on_validator(d, presentC)
		errorC2 <- in.errorC
	}
	if <-presentC {

		<-errorC2
		return
	}
	errorC := <-errorC2
	doneC := e1.ctx.Done()
	slot := uint64(0)
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:
			if slot != 0 && finish <= slot {
				select {
				case <-doneC:
					break out
				case e1.updateValidatorStakeC <- validatorStakeUpdate{
					vote: vote,
					status: val.StakeStatus{
						Activated: 0,
						Total:     0,
					},
					start:  start,
					finish: finish,
				}:
				}
			}
		case err = <-sub.ErrorC:
			break out
		case s := <-sub.StreamC:
			select {
			case <-doneC:
				break out
			case e1.updateValidatorStakeC <- validatorStakeUpdate{
				vote:   vote,
				status: s,
				start:  start,
				finish: finish,
			}:
			}

		}
	}
	if err != nil {
		errorC <- err
	}
}
