package pricing

func (in *internal) on_period(update periodUpdate) {
	if update.data.Period.IsBlank {
		return
	}
	_, present := in.payoutM[update.payout.Id.String()]
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

	in.payoutM[update.payout.Id.String()] = &periodInfo{
		period:       update.data.Period,
		bs:           bs,
		pipelineInfo: pi,
	}
}
