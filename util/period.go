package util

import (
	"errors"

	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
)

func GetLinkedListFromPeriodRing(periods *cba.PeriodRing) (*ll.Generic[cba.PeriodWithPayout], error) {
	list := ll.CreateGeneric[cba.PeriodWithPayout]()
	if periods == nil {
		return nil, errors.New("no period rings")
	}
	if periods.Ring == nil {
		return nil, errors.New("no period rings")
	}

	for i := uint16(0); i < periods.Length; i++ {
		k := (periods.Start + i) % uint16(len(periods.Ring))
		list.Append(periods.Ring[k])
	}
	return list, nil
}
