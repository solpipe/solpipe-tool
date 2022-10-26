package script

import (
	"errors"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

func (e1 *Script) CreateController(adminKey sgo.PrivateKey, crankAuthority sgo.PrivateKey, mint sgo.PublicKey, feeRate *state.Rate) error {
	if e1.txBuilder == nil {
		return errors.New("tx builder is nil")
	}
	if feeRate == nil {
		return errors.New("no fee rate")
	}
	log.Debugf("fee rate=%+v", feeRate)

	controllerId, _, err := vrs.ControllerId(e1.config.Version)
	if err != nil {
		return err
	}

	pcVault, _, err := vrs.PcVaultId(e1.config.Version)
	if err != nil {
		return err
	}
	log.Infof("controller id=%s", controllerId.String())
	log.Infof("pc vault=%s", pcVault.String())
	log.Infof("mint=%s", mint.String())
	log.Infof("crankAuthority=%s", mint.String())
	// keyMap[adminKey.PublicKey().String()] = adminKey

	i := cba.NewCreateInstructionBuilder()
	i.SetAdminAccount(adminKey.PublicKey())
	e1.AppendKey(adminKey)
	i.SetControllerAccount(controllerId)
	i.SetCrankAuthorityAccount(crankAuthority.PublicKey())
	e1.AppendKey(crankAuthority)
	i.SetPcMintAccount(mint)
	i.SetPcVaultAccount(pcVault)
	i.SetRentAccount(sgo.SysVarRentPubkey)
	i.SetClockAccount(sgo.SysVarClockPubkey)
	i.SetSystemProgramAccount(sgo.SystemProgramID)
	i.SetTokenProgramAccount(sgo.TokenProgramID)
	i.SetEpochScheduleAccount(sgo.SysVarEpochSchedulePubkey)

	i.SetControllerFeeNum(feeRate.N)
	i.SetControllerFeeDen(feeRate.D)

	e1.txBuilder.AddInstruction(i.Build())

	return nil

}
