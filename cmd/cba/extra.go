package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/solpipe/solpipe-tool/state"
)

type ConfigRate state.Rate

func (rate *ConfigRate) UnmarshalText(data []byte) error {

	x := strings.Split(string(data), "/")
	if len(x) != 2 {
		return errors.New("rate should be if form uint64/uint64")
	}
	num, err := strconv.ParseUint(x[0], 10, 8*8)
	if err != nil {
		return err
	}
	den, err := strconv.ParseUint(x[1], 10, 8*8)
	if err != nil {
		return err
	}
	rate.N = num
	rate.D = den
	return nil
}

func convertRate(in string, defaultRate *state.Rate) (*state.Rate, error) {
	var pair [2]uint64
	var err error

	if 0 < len(in) {
		split := strings.Split(in, "/")
		if len(split) != 2 {
			return nil, errors.New("format not N/D")
		}

		for i := 0; i < 2; i++ {
			pair[i], err = strconv.ParseUint(split[i], 10, 8)
			if err != nil {
				return nil, err
			}
		}
	}

	if pair[1] == 0 {
		return defaultRate, nil
	} else {
		return &state.Rate{
			N: pair[0],
			D: pair[1],
		}, nil
	}

}
