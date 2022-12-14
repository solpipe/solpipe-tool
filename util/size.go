package util

import "errors"

const (
	STRUCT_SIZE_MINT            uint64 = 82
	STRUCT_SIZE_CONTROLLER      uint64 = 184
	STRUCT_SIZE_PIPELINE        uint64 = 353
	STRUCT_SIZE_PERIOD_RING     uint64 = 5000
	STRUCT_SIZE_BID_LIST        uint64 = 1600000
	STRUCT_SIZE_BID_LIST_HEADER uint64 = 1 + 1 + 40 + 8 + 8 + 8
	STRUCT_SIZE_REFUND_HEADER   uint64 = 40 + 8
	STRUCT_SIZE_REFUND_CLAIM    uint64 = 40 + 8
	STRUCT_BID_SINGLE           uint64 = 1 + 40 + 8 + 8
)

/*
length=1600000 and discriminator=e97f0d1d7bd1c04f"
time="2022-07-23T11:05:42+09:00" level=debug msg="....length=353 and discriminator=1e5210dac44d73e0"
time="2022-07-23T11:05:42+09:00" level=debug msg="....length=5000 and discriminator=3dbf3b8fe2eb681a"
*/

type Delta struct {
	Change     uint64
	IsIncrease bool
}

// calculate the delta between the o=old value and n=new value
func CalculateDelta(o uint64, n uint64) Delta {
	var delta uint64
	var isAddition bool
	if o < n {
		isAddition = true
		delta = n - o
	} else {
		isAddition = false
		delta = o - n
	}
	return Delta{
		Change:     delta,
		IsIncrease: isAddition,
	}
}

const (
	MAX_PC uint64 = 100_000_000_000_000
)

func SafeConvertUIntToFloat(x uint64) (float64, error) {
	if MAX_PC <= x {
		return 0, errors.New("integer out of range")
	} else {
		return float64(x), nil
	}
}
