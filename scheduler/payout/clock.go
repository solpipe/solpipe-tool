package payout

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
)

// this is from the Solana Program itself
const PAYOUT_POST_FINISH_DELAY uint64 = 100

func loopClock(
	ctx context.Context,
	controller ctr.Controller,
	eventC chan<- sch.Event,
	errorC chan<- error,
	clockPeriodStartC chan<- bool,
	clockPeriodPostC chan<- bool,
	data cba.Payout,
	payoutId sgo.PublicKey,
) {
	var err error
	var slot uint64
	doneC := ctx.Done()
	slotSub := controller.SlotHome().OnSlot()
	defer slotSub.Unsubscribe()
	sentPreStart := false
	start := data.Period.Start
	sentStart := false
	isStartStateTransition := false
	finish := start + data.Period.Length - 1
	sentFinish := false
	isFinishStateTransition := false
	closeOut := finish + PAYOUT_POST_FINISH_DELAY
	sentClose := false
	isCloseStateTransition := false

out:
	for !sentClose {
		select {
		case <-doneC:
			break out
		case err = <-slotSub.ErrorC:
			break out
		case slot = <-slotSub.StreamC:

			if !sentStart && start <= slot {
				log.Debugf("SENDING+++++ EVENT_PERIOD_START payout=%s", payoutId.String())
				sentStart = true
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(sch.EVENT_PERIOD_START, isStartStateTransition, slot):
				}
			} else if !isStartStateTransition {
				isStartStateTransition = true
			}
			if !sentPreStart && slot < start {
				sentPreStart = true
				log.Debugf("SENDING+++++ EVENT_PERIOD_PRE_START payout=%s", payoutId.String())
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(sch.EVENT_PERIOD_PRE_START, true, slot):
				}
			}
			if !sentFinish && finish <= slot {
				log.Debugf("SENDING+++++ EVENT_PERIOD_FINISH payout=%s", payoutId.String())
				sentFinish = true
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(sch.EVENT_PERIOD_FINISH, isFinishStateTransition, slot):
				}
			} else if !isFinishStateTransition {
				isFinishStateTransition = true
			}
			if !sentClose && closeOut <= slot {
				sentClose = true
				select {
				case <-doneC:
					break out
				case eventC <- sch.Create(sch.EVENT_DELAY_CLOSE_PAYOUT, isCloseStateTransition, slot):
				}
				select {
				case <-doneC:
					break out
				case clockPeriodPostC <- isCloseStateTransition:
				}
			} else if !isCloseStateTransition {
				isCloseStateTransition = true
			}
		}
	}
	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}
