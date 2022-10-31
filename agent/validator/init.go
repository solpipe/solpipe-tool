package validator

import (
	"context"
	"errors"
	"net/http"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type InitializationArg struct {
	Version        vrs.CbaVersion
	Wallet         sgo.PrivateKey
	Admin          sgo.PrivateKey
	ControllerId   sgo.PublicKey
	Vote           sgo.PrivateKey
	Stake          sgo.PublicKey
	RpcUrl         string
	WsUrl          string
	AdminListenUrl string
	Headers        http.Header
}

func (args *InitializationArg) RelayConfig() relay.Configuration {

	return relay.CreateConfiguration(
		args.Version,
		args.Admin,
		args.RpcUrl,
		args.WsUrl,
		args.Headers,
		args.AdminListenUrl,
		nil,
	)
}

const VALIDATOR_MEMBER_SIZE = 500

// Run the AddValidator instruction
func Initialize(
	ctx context.Context,
	router rtr.Router,
	timeout time.Duration,
	args *InitializationArg,
) (l ListenResult, err error) {

	if args == nil {
		err = errors.New("no config")
		return
	}
	config := args.RelayConfig()
	rpcClient := config.Rpc()
	wsClient, err := config.Ws(ctx)
	if err != nil {
		return
	}

	minSize := uint64(VALIDATOR_MEMBER_SIZE)
	minRent, err := rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		minSize,
		sgorpc.CommitmentFinalized,
	)
	if err != nil {
		return
	}
	var adminBalance uint64
	{
		var x *sgorpc.GetBalanceResult
		x, err = rpcClient.GetBalance(ctx, args.Admin.PublicKey(), sgorpc.CommitmentFinalized)
		if err != nil {
			return
		}
		adminBalance = x.Value
	}

	s1, err := script.Create(ctx, &script.Configuration{Version: args.Version}, rpcClient, wsClient)
	if err != nil {
		return
	}
	err = s1.SetTx(args.Wallet)
	if err != nil {
		return
	}
	if adminBalance < minRent {
		s1.Transfer(args.Wallet, args.Admin.PublicKey(), minRent-adminBalance)
	}
	_, err = s1.AddValidator(args.ControllerId, args.Vote, args.Stake, args.Admin)
	if err != nil {
		return
	}

	l = CreateFromListener(
		ctx,
		config,
		router,
		args.Vote.PublicKey(),
		timeout,
	)

	err = s1.FinishTx(true)
	if err != nil {
		return
	}
	return

}
