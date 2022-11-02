package main

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	bin "github.com/gagliardetto/binary"
	bdr "github.com/solpipe/solpipe-tool/agent/bidder"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"

	sgotkn2 "github.com/SolmateDev/solana-go/programs/token"
)

type Bidder struct {
	Setup BidderSetup `cmd name:"setup" help:"create token accounts to bid on space"`
	Agent BidderAgent `cmd name:"agent" help:"run a Bidding Agent"`
}

type BidderSetup struct {
}

func (r *BidderSetup) Run(kongCtx *CLIContext) error {
	return errors.New("not implemented yet")
}

type BidderAgent struct {
	Key           string `arg name:"key" help:"the private key of the wallet that owns tokens used to bid on bandwidth and also authenticates over grpc with the staked validator"`
	Configuration string `arg name:"config" help:"the file path to the configuration file"`
}

func (r *BidderAgent) Run(kongCtx *CLIContext) error {

	ctx := kongCtx.Ctx
	admin, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Key)
	if err != nil {
		return err
	}

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		admin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		"",
		nil,
	)

	rpcClient := relayConfig.Rpc()
	wsClient, err := relayConfig.Ws(ctx)
	if err != nil {
		return err
	}
	controller, err := ctr.CreateController(ctx, rpcClient, wsClient, relayConfig.Version)
	if err != nil {
		return err
	}
	network, err := ntk.Create(ctx, controller, rpcClient, wsClient)
	if err != nil {
		return err
	}

	userKey, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Key)
	if err != nil {
		return err
	}
	router, err := rtr.CreateRouter(ctx, network, rpcClient, wsClient, nil, relayConfig.Version)
	if err != nil {
		return err
	}

	controllerData, err := controller.Data()
	if err != nil {
		return err
	}

	pcMint := controllerData.PcMint
	x, err := rpcClient.GetTokenAccountsByOwner(
		ctx,
		userKey.PublicKey(),
		&sgorpc.GetTokenAccountsConfig{Mint: &pcMint},
		&sgorpc.GetTokenAccountsOpts{Commitment: sgorpc.CommitmentFinalized},
	)
	if err != nil {
		return err
	}
	var userVault *sgotkn2.Account
	var userVaultId sgo.PublicKey
backout:
	for i := 0; i < len(x.Value); i++ {
		userVault = new(sgotkn2.Account)
		err = bin.UnmarshalBorsh(userVault, x.Value[i].Account.Data.GetBinary())
		if err != nil {
			return err
		}
		if userVault.Mint.Equals(controllerData.PcMint) {
			userVaultId = x.Value[i].Pubkey
			break backout
		}
	}
	if userVault == nil {
		return errors.New("failed to find user vault")
	}

	configChannelGroup := bdr.ConfigByHup(ctx, r.Configuration)
	bidder, err := bdr.Create(
		ctx,
		rpcClient,
		wsClient,
		configChannelGroup,
		userKey,
		userVaultId,
		userVault,
		router,
	)
	if err != nil {
		return err
	}
	err = <-bidder.CloseSignal()
	if err != nil {
		return err
	}
	return nil
}
