package admin

import log "github.com/sirupsen/logrus"

// when is the earliest slot to which a period can be appended
func (in *internal) next_slot() uint64 {
	openSlot := in.slot
	tail, present := in.payoutInfo.list.Tail()
	if present {
		// check if the next_slot is in the future
		if in.slot < tail.pwd.Data.Period.Start+tail.pwd.Data.Period.Length {
			openSlot = tail.pwd.Data.Period.Start + tail.pwd.Data.Period.Length
		}
	}
	return openSlot
}

/*
func (in *internal) on_period2(list *ll.Generic[cba.PeriodWithPayout]) {

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
*/

const RETRY_INTERVAL = uint64(50)

func (in *internal) on_slot(slot uint64) {
	in.slot = slot

	tail := in.payoutInfo.tailSlot // start+length
	lookaheadLimit := slot + in.periodSettings.Lookahead
	retryLimit := in.lastAddPeriodAttemptToAddPeriod + RETRY_INTERVAL

	start := tail
	if start < slot {
		start = slot
	}
	if !in.appendInProgress && retryLimit < slot && start < lookaheadLimit {
		log.Debugf("slot=%d; tail=%d; retry=%d; start=%d; lookahead=%d;", slot, tail, retryLimit, start, lookaheadLimit)
		in.attempt_add_period(start)
	}
}
