package router

import (
	"context"

	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	bin "github.com/gagliardetto/binary"
)

func GetPeriodRing(ctx context.Context, rpcClient *sgorpc.Client, router Router) error {

	r, err := rpcClient.GetProgramAccountsWithOpts(ctx, cba.ProgramID, &sgorpc.GetProgramAccountsOpts{
		Commitment: sgorpc.CommitmentConfirmed,
		Encoding:   sgo.EncodingBase64,
		//Filters: []sgorpc.RPCFilter{
		//	{
		//		Memcmp: &sgorpc.RPCFilterMemcmp{
		//			Offset: 0, Bytes: []byte(cba.PeriodRingDiscriminator[:]),
		//		},
		//	},
		//},
	})
	if err != nil {
		return err
	}
	for i := 0; i < len(r); i++ {
		data := r[i].Account.Data.GetBinary()
		if util.Compare(data[0:8], cba.PeriodRingDiscriminator[0:8]) {
			ring := new(cba.PeriodRing)
			err = bin.UnmarshalBorsh(ring, r[i].Account.Data.GetBinary())
			if err != nil {
				return err
			}
			err = router.AddPeriod(*ring)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

func GetBidList(ctx context.Context, rpcClient *sgorpc.Client, router Router) error {
	//prefix := sgo.Base58(base58.Encode(cba.BidListDiscriminator[:]))
	r, err := rpcClient.GetProgramAccountsWithOpts(ctx, cba.ProgramID, &sgorpc.GetProgramAccountsOpts{
		Commitment: sgorpc.CommitmentConfirmed,
		Encoding:   sgo.EncodingBase64,
		//Filters: []sgorpc.RPCFilter{
		//	{
		//		Memcmp: &sgorpc.RPCFilterMemcmp{
		//			Offset: 0, Bytes: prefix,
		//		},
		//	},
		//},
	})
	if err != nil {
		return err
	}
	for i := 0; i < len(r); i++ {
		data := r[i].Account.Data.GetBinary()
		//if data[0:8]!=cba.BidListDiscriminator[0:8]{
		if util.Compare(data[0:8], cba.BidListDiscriminator[0:8]) {
			list := new(cba.BidList)
			err = bin.UnmarshalBorsh(list, data)
			if err != nil {
				return err
			}
			err = router.AddBid(*list)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
