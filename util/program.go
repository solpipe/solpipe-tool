package util

import (
	"errors"
	"os"

	cba "github.com/solpipe/cba"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

func SetProgramID() error {
	idStr, present := os.LookupEnv("PROGRAM_ID_CBA")
	if present {
		id, err := sgo.PublicKeyFromBase58(idStr)
		if err != nil {
			return err
		}
		cba.SetProgramID(id)
		log.Debugf("pub program id__=%s", id.String())
	} else {
		fp, present := os.LookupEnv("PROGRAM_ID_CBA_KEYPAIR")
		if present {
			key, err := sgo.PrivateKeyFromSolanaKeygenFile(fp)
			if err != nil {
				return err
			}
			log.Debugf("kp program id__=%s", key.PublicKey().String())
			cba.SetProgramID(key.PublicKey())
		} else {
			return errors.New("no program id")
		}
	}
	return nil
}
