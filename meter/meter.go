package meter

import (
	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
)

// Pipeline record usage
type Recorder interface {
	// this is either the Bidder or Validator
	CounterParty() sgo.PublicKey
	Append([]SingleRecord) (ServiceSummary, error)
	AllRecords() ([]SingleRecord, error)
	Summary() ServiceSummary
	OnSummary() dssub.Subscription[ServiceSummary]
}

type SingleRecord struct {
	Signature sgo.Hash
}

type ServiceSummary struct {
	MsgCounter uint32
	Signed     bool
	MerkleRoot sgo.Hash
}

func SummaryFromReceipt(r cba.Receipt) ServiceSummary {
	return ServiceSummary{
		MsgCounter: r.MsgCounter,
		Signed:     r.MsgCounter == r.ValidatorSigned,
		MerkleRoot: r.TxSentMerkleroot,
	}
}

// TODO: add approver
