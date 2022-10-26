package version

import (
	"errors"

	cba "github.com/solpipe/cba"
	sgo "github.com/SolmateDev/solana-go"
)

type CbaVersion string

const VERSION_1 CbaVersion = "1"

func ControllerId(version CbaVersion) (sgo.PublicKey, uint8, error) {
	if version != VERSION_1 {
		return sgo.PublicKey{}, 0, errors.New("bad version")
	}
	name := "controller"
	return sgo.FindProgramAddress([][]byte{[]byte(name[:])}, cba.ProgramID)
}

func PcVaultId(version CbaVersion) (sgo.PublicKey, uint8, error) {
	name := "pc_vault"
	return sgo.FindProgramAddress([][]byte{[]byte(name[:])}, cba.ProgramID)
}

func PipelineVaultId(version CbaVersion, pipelineId sgo.PublicKey) (sgo.PublicKey, uint8, error) {
	// "pc_vault",pipeline.key().as_ref()
	name := "pc_vault"
	return sgo.FindProgramAddress([][]byte{[]byte(name[:]), pipelineId.Bytes()}, cba.ProgramID)
}
