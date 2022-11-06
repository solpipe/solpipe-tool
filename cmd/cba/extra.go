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
