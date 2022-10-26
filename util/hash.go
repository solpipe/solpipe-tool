package util

import (
	"crypto/sha256"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
)

func HashTransaction(tx *sgo.Transaction) (hash sgo.Hash, err error) {
	if tx == nil {
		err = errors.New("blank transaction")
		return
	}
	err = tx.VerifySignatures()
	if err != nil {
		return
	}
	var data []byte
	data, err = tx.MarshalBinary()
	if err != nil {
		return
	}
	kh := sha256.New()
	kh.Write(data)
	out := kh.Sum(nil)
	if len(out) != sgo.PublicKeyLength {
		err = errors.New("hash is wrong size")
	}
	hash = sgo.HashFromBytes(out)
	return
}
