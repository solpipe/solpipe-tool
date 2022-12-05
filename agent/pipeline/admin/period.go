package admin

import (
	"errors"
	"math"

	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

func (in *internal) attempt_add_period() error {
	start := in.next_slot()
	log.Debugf("attempting to add a period; start=%d; end=%d; bid space=%d", start, start+in.periodSettings.Length-1, in.periodSettings.BidSpace)
	if math.MaxUint16 <= in.periodSettings.Withhold {
		return errors.New("withhold is too large")
	}
	if math.MaxUint16 <= in.periodSettings.BidSpace {
		return errors.New("bid space is too large")
	}
	if in.periodSettings.BidSpace <= 10 {
		return errors.New("bid space is too small")
	}
	s1, err := script.Create(in.ctx, &script.Configuration{Version: in.controller.Version}, in.rpc, in.ws)
	if err != nil {
		return err
	}
	err = s1.SetTx(in.admin)
	if err != nil {
		return err
	}
	withhold := uint16(in.periodSettings.Withhold)
	err = s1.UpdatePipeline(
		in.controller.Id(),
		in.pipeline.Id,
		in.admin,
		state.RateFromProto(in.rateSettings.CrankFee),
		pipe.ALLOTMENT_DEFAULT,
		state.RateFromProto(in.rateSettings.PayoutShare),
		uint16(in.periodSettings.TickSize),
	)
	if err != nil {
		return err
	}

	_, err = s1.AppendPeriod(
		in.controller,
		in.pipeline,
		in.admin,
		start,
		in.periodSettings.Length,
		withhold,
		uint16(in.periodSettings.BidSpace),
	)
	if err != nil {
		return err
	}

	go loopAttempAppendPeriod(s1, in.lastAddPeriodAttemptToAddPeriod, in.addPeriodResultC)

	return nil
}

func loopAttempAppendPeriod(s1 *script.Script, attempt uint64, resultC chan<- addPeriodResult) {
	err := s1.FinishTx(true)
	resultC <- addPeriodResult{err: err, attempt: attempt}
}
