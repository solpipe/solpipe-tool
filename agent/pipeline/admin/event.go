package admin

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
)

// when is the earliest slot to which a period can be appended
func (in *internal) next_slot() uint64 {
	openSlot := in.slot
	tail, present := in.list.Tail()
	if present {
		if in.slot < tail.Period.Start+tail.Period.Length {
			openSlot = tail.Period.Start + tail.Period.Length
		}
	}
	return openSlot
}

// Set this before doing an append-attempt so as to not immediately do another attempt while
// still waiting for the original attempt to return a result.
// newPeriod=true means that we ignore the RETRY_INTERVAL
// either append the new period at the end of the tail period, or at the current slot.
func (in *internal) calculate_next_attempt_to_add_period(newPeriod bool) {

	var nextAttempt uint64
	nextSlot := in.next_slot()
	lookAheadLimit := in.slot + in.periodSettings.Lookahead

	if newPeriod && lookAheadLimit < nextSlot {
		nextAttempt = in.slot + nextSlot - lookAheadLimit
		log.Debugf("(slot=%d) next attempt must wait, too far ahead: (attempt=%d,nextSlot=%d,lookaheadlimit=%d)", in.slot, nextAttempt, nextSlot, lookAheadLimit)
	} else if newPeriod {
		nextAttempt = in.slot
		log.Debugf("(slot=%d) next attempt immediate: (attempt=%d,nextSlot=%d,lookaheadlimit=%d)", in.slot, nextAttempt, nextSlot, lookAheadLimit)
	} else {
		if nextAttempt != in.lastAddPeriodAttemptToAddPeriod+RETRY_INTERVAL {
			log.Debugf("(slot=%d) next attempt retry: (attempt=%d,last=%d,retry=%d)", in.slot, nextAttempt, in.lastAddPeriodAttemptToAddPeriod, RETRY_INTERVAL)
		}
		nextAttempt = in.lastAddPeriodAttemptToAddPeriod + RETRY_INTERVAL
	}
	if in.nextAttemptToAddPeriod != nextAttempt {
		log.Debugf("next attempt %d", nextAttempt)
		in.nextAttemptToAddPeriod = nextAttempt
	}
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
	if 0 < in.nextAttemptToAddPeriod && in.nextAttemptToAddPeriod <= in.slot {
		in.lastAddPeriodAttemptToAddPeriod = in.slot
		in.calculate_next_attempt_to_add_period(false)
		err = in.attempt_add_period()
		if err != nil {
			in.addPeriodResultC <- addPeriodResult{err: err, attempt: in.slot}
		}
	} else if in.nextAttemptToAddPeriod == 0 {
		in.calculate_next_attempt_to_add_period(false)
	}
}
