package sub

import (
	"math/big"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
)

func GetBidSummary(list *cba.BidList) *BidSummary {
	ans := new(BidSummary)
	ans.Payout = list.Payout
	ans.TotalDeposits = 0
	for i := 0; i < len(list.Book); i++ {
		ans.TotalDeposits += list.Book[i].Deposit
	}
	return ans
}

type ValidatorGroup struct {
	Id     sgo.PublicKey
	Data   cba.ValidatorManager
	IsOpen bool
}

type PipelineGroup struct {
	Id     sgo.PublicKey
	Data   cba.Pipeline
	IsOpen bool
}

type BidGroup struct {
	Id     sgo.PublicKey
	Data   cba.BidList
	IsOpen bool
}

type PeriodGroup struct {
	Id     sgo.PublicKey
	Data   cba.PeriodRing
	IsOpen bool
}

type BidForBidder struct {
	// period
	LastPeriodStart uint64
	// the validator to which this bid belongs to
	Pipeline sgo.PublicKey
	// actual bid
	Bid cba.Bid
}

type BidSummary struct {
	// the validator to which this bid belongs to
	Payout sgo.PublicKey
	// total deposits in the bid bucket
	TotalDeposits uint64
}

type StakeGroup struct {
	Id     sgo.PublicKey
	Data   cba.StakerManager
	IsOpen bool
}

type StakerReceiptGroup struct {
	Id     sgo.PublicKey
	Data   cba.StakerReceipt
	IsOpen bool
}

type ReceiptGroup struct {
	Id     sgo.PublicKey
	Data   cba.Receipt
	IsOpen bool
}

type PayoutWithData struct {
	Id     sgo.PublicKey
	Data   cba.Payout
	IsOpen bool
}

type StakeUpdate struct {
	ActivatedStake *big.Int
	TotalStake     *big.Int
}

// what is the alloted tps
func (tu StakeUpdate) Tps(networkTps *big.Float) *big.Float {
	activatedStake := big.NewFloat(0)
	activatedStake.SetInt(tu.ActivatedStake)
	totalStake := big.NewFloat(0)
	totalStake.SetInt(tu.TotalStake)
	if tu.TotalStake == big.NewInt(0) {
		return big.NewFloat(0)
	}
	ans := big.NewFloat(0)
	ans.Mul(networkTps, activatedStake)
	return ans.Quo(ans, totalStake)
}
