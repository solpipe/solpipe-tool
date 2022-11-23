package util

import (
	pba "github.com/solpipe/solpipe-tool/proto/admin"
)

func CopyValidatorSettings(a *pba.ValidatorSettings) *pba.ValidatorSettings {
	return &pba.ValidatorSettings{
		PipelineId: a.GetPipelineId(),
	}
}

func CopyPeriodSettings(a *pba.PeriodSettings) *pba.PeriodSettings {
	return &pba.PeriodSettings{
		Withhold:  a.Withhold,
		Lookahead: a.Lookahead,
		Length:    a.Length,
	}
}

func CopyRate(a *pba.Rate) *pba.Rate {
	return &pba.Rate{
		Numerator: a.Numerator, Denominator: a.Denominator,
	}
}
