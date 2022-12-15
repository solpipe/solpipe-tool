package pricing

// when do we do adjustments before a period starts
func COUNT_DOWN() []uint64 {
	return []uint64{1000, 250, 100}
}

type CapacityPoint struct {
	Start uint64  // slot
	Tps   float64 // required Tps
}

func (pr *periodInfo) calculate_tps() (float64, error) {
	share, err := pr.bs.Share(pr.pipelineInfo.bidder)
	if err != nil {
		return 0, err
	}
	return pr.pipelineInfo.stats.tps * share, nil
}

func (in *internal) update_tps(pr *periodInfo) error {
	var err error
	pr.requiredTps, err = in.capacity_requirement(pr)
	if err != nil {
		return err
	}
	pr.projectedTps, err = pr.calculate_tps()
	if err != nil {
		return err
	}
	return nil
}
