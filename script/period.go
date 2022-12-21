package script

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

// get the start=end_of_last_period from either pipeline.PeriodRing() or pipeline.OnPeriod()
func (e1 *Script) AppendPeriod(
	controller ctr.Controller,
	pipeline pipe.Pipeline,
	admin sgo.PrivateKey,
	start uint64,
	length uint64,
	withhold uint16,
	bidSpace uint16,
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

	bidListId, err := e1.CreateAccount(e1.bidListSize(bidSpace), cba.ProgramID, admin)
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
	b.SetBidsAccount(bidListId)
	e1.AppendKey(payout)
	b.SetPeriodsAccount(data.Periods)
	b.SetPipelineAccount(pipeline.Id)
	b.SetRentAccount(sgo.SysVarRentPubkey)
	b.SetSystemProgramAccount(sgo.SystemProgramID)
	b.SetTokenProgramAccount(sgo.TokenProgramID)

	log.Debugf("start=%d", start)
	b.SetStart(start)
	log.Debugf("length=%d", length)
	b.SetLength(length)
	log.Debugf("withhold=%d", withhold)
	b.SetWithhold(withhold)
	log.Debugf("bid space=%d", bidSpace)
	b.SetBidSpace(bidSpace)

	e1.txBuilder.AddInstruction(b.Build())

	return
}
