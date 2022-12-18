package admin

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

func (in *internal) on_slot(slot uint64) {
	in.slot = slot
	in.attempt_add_period()
}
