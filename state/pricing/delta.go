package pricing

import "time"

// when do we do adjustments before a period starts
func COUNT_DOWN() []uint64 {
	return []uint64{1000, 250, 100}
}

type CapacityPoint struct {
	Time time.Time
	Tps  float64 // required Tps
}

func (pi *pipelineInfo) delta() {

}
