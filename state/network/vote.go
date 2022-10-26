package network

import (
	"context"
	"time"

	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
)

type VoteStake struct {
	Id             sgo.PublicKey
	ActivatedStake uint64
}

func ZeroPub() sgo.PublicKey {
	zero := make([]byte, sgo.PublicKeyLength)
	return sgo.PublicKeyFromBytes(zero)
}

func loopVoteUpdate(
	ctx context.Context,
	cancel context.CancelFunc,
	rpcClient *sgorpc.Client,
	voteHome *sub2.SubHome[VoteStake],
) {
	defer cancel()
	doneC := ctx.Done()
	delay := 5 * time.Second

	zero := ZeroPub()
out:
	for {
		select {
		case <-doneC:
			break out
		case id := <-voteHome.DeleteC:
			voteHome.Delete(id)
		case r := <-voteHome.ReqC:
			voteHome.Receive(r)
		case <-time.After(delay):
			delay = 5 * time.Minute
			r, err := rpcClient.GetVoteAccounts(ctx, &sgorpc.GetVoteAccountsOpts{
				Commitment: sgorpc.CommitmentFinalized,
			})
			if err == nil {
				totalStake := uint64(0)
				for i := 0; i < len(r.Current); i++ {
					voteHome.Broadcast(VoteStake{
						Id:             r.Current[i].VotePubkey,
						ActivatedStake: r.Current[i].ActivatedStake,
					})
					totalStake += r.Current[i].ActivatedStake
				}
				voteHome.Broadcast(VoteStake{
					Id:             zero,
					ActivatedStake: totalStake,
				})
			}

		}
	}

}
