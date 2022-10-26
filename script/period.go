package script

import (
	"errors"

	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

// get the start=end_of_last_period from either pipeline.PeriodRing() or pipeline.OnPeriod()
func (e1 *Script) AppendPeriod(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	admin sgo.PrivateKey,
	start uint64,
	length uint64,
	withhold uint16,
) (payoutId sgo.PublicKey, err error) {
	if e1.txBuilder == nil {
		err = errors.New("no tx builder")
		return
	}
	data, err := pipeline.Data()
	if err != nil {
		return
	}

	payout, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return
	}
	payoutId = payout.PublicKey()
	log.Debugf("controller=%s", controller.Id().String())
	log.Debugf("payout=%s", payout.PublicKey())
	log.Debugf("pipeline=%s", pipeline.Id.String())
	log.Debugf("period=%s", data.Periods.String())
	b := cba.NewAppendPeriodInstructionBuilder()
	b.SetAdminAccount(admin.PublicKey())
	b.SetClockAccount(sgo.SysVarClockPubkey)
	b.SetControllerAccount(controller.Id())
	b.SetEpochScheduleAccount(sgo.SysVarEpochSchedulePubkey)
	b.SetPayoutAccount(payout.PublicKey())
	e1.AppendKey(payout)
	b.SetPeriodsAccount(data.Periods)
	b.SetPipelineAccount(pipeline.Id)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetTokenProgramAccount(sgo.TokenProgramID)

	b.SetStart(start)
	b.SetLength(length)
	b.SetWithhold(withhold)

	e1.txBuilder.AddInstruction(b.Build())

	return
}
