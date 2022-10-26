package admin

import (
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	log "github.com/sirupsen/logrus"
)

func (in *internal) calculate_next_attempt_to_add_period(newPeriod bool) {

	nextAttempt := in.slot
	tail, present := in.list.Tail()
	if present {
		if in.slot < tail.Period.Start+tail.Period.Length && tail.Period.Start+tail.Period.Length < in.periodSettings.Lookahead {
			nextAttempt = tail.Period.Start + tail.Period.Length
			log.Debugf("next attempt by period %d", nextAttempt)
		}
	}
	if !newPeriod {
		if nextAttempt < in.lastAddPeriodAttemptToAddPeriod+RETRY_INTERVAL {
			nextAttempt = in.lastAddPeriodAttemptToAddPeriod + RETRY_INTERVAL
			log.Debugf("next attempt by retry %d", nextAttempt)
		}
	}

	log.Debugf("calculate next attempt period (old=%d vs new=%d)", in.nextAttemptToAddPeriod, nextAttempt)
	in.nextAttemptToAddPeriod = nextAttempt
}

func (in *internal) on_period(list *ll.Generic[cba.PeriodWithPayout]) {

	if list != nil {
		in.list = list
	}
	log.Debugf("new period ring length=%d", in.list.Size)

	list.Iterate(func(obj cba.PeriodWithPayout, index uint32, delete func()) error {
		log.Debugf("period rings:______(start=%d; finish=%d; payout=%s)", obj.Period.Start, obj.Period.Start+obj.Period.Length, obj.Payout.String())
		return nil
	})
	in.calculate_next_attempt_to_add_period(true)

}

const RETRY_INTERVAL = uint64(50)

func (in *internal) on_slot(slot uint64) {
	in.slot = slot
	var err error
	if in.nextAttemptToAddPeriod <= in.slot {
		in.lastAddPeriodAttemptToAddPeriod = in.slot
		in.calculate_next_attempt_to_add_period(false)
		err = in.attempt_add_period(in.slot)
		if err != nil {
			in.addPeriodResultC <- addPeriodResult{err: err, attempt: in.slot}
		}
	}
}
