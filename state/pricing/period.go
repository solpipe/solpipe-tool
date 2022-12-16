package pricing

func (pi *periodInfo) start() uint64 {
	return pi.period.Start
}
func (pi *periodInfo) finish() uint64 {
	return pi.period.Start + pi.period.Length - 1
}

func (in *internal) on_period(update periodUpdate) {
	if update.data.Period.IsBlank {
		return
	}
	pr, present := in.payoutM[update.payout.Id.String()]
	if present {
		return
	}
	pi, present := in.pipelineM[update.pipelineId.String()]
	if !present {
		return
	}

	bs, err := update.payout.BidStatus()
	if err != nil {
		in.errorC <- err
		return
	}
	pr = &periodInfo{
		period:       update.data.Period,
		bs:           bs,
		pipelineInfo: pi,
	}
	in.payoutM[update.payout.Id.String()] = pr
}
