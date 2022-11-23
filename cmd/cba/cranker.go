package main

import (
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	log "github.com/sirupsen/logrus"
	ckr "github.com/solpipe/solpipe-tool/agent/cranker"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Cranker struct {
	BalanceThreshold uint64 `arg name:"minbal" help:"what is the balance threshold at which the program needs to exit with an error code"`
	Key              string `arg name:"key" help:"the file path of the private key"`
}

func (r *Cranker) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx

	crankerKey, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Key)
	if err != nil {
		return err
	}

	log.Infof("ws url=%s", kongCtx.Clients.WsUrl)

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		crankerKey,
		string(kongCtx.Clients.RpcUrl),
		string(kongCtx.Clients.WsUrl),
		kongCtx.Clients.Headers.Clone(),
		"",
		nil,
	)

	rpcClient := relayConfig.Rpc()
	wsClient, err := relayConfig.Ws(ctx)
	if err != nil {
		return err
	}
	controller, err := ctr.CreateController(
		ctx, rpcClient, wsClient, relayConfig.Version,
	)
	if err != nil {
		return err
	}

	network, err := ntk.Create(ctx, controller, rpcClient, wsClient)
	if err != nil {
		return err
	}

	router, err := rtr.CreateRouter(ctx, network, rpcClient, wsClient, nil, relayConfig.Version)
	if err != nil {
		return err
	}

	if r.BalanceThreshold == 0 {
		r.BalanceThreshold = 1 * sgo.LAMPORTS_PER_SOL
	}

	{
		d, err := router.Controller.Data()
		if err != nil {
			return err
		}
		address, _, err := sgo.FindAssociatedTokenAddress(crankerKey.PublicKey(), d.PcMint)
		if err != nil {
			return err
		}
		createNewAccount := false
		bh, err := rpcClient.GetBalance(ctx, address, sgorpc.CommitmentFinalized)
		if err != nil {
			createNewAccount = true
		} else if bh.Value == 0 {
			createNewAccount = true
		}
		if createNewAccount {
			log.Debug("creating token account for cranker")
			s, err := relayConfig.ScriptBuilder(ctx)
			if err != nil {
				return err
			}
			err = s.CreateTokenAccount(relayConfig.Admin, relayConfig.Admin.PublicKey(), d.PcMint)
			if err != nil {
				return err
			}
			err = s.FinishTx(true)
			if err != nil {
				return err
			}
		}
	}

	cranker, err := ckr.Create(
		ctx,
		&relayConfig,
		r.BalanceThreshold,
		router,
	)
	if err != nil {
		return err
	}

	return <-cranker.CloseSignal()
}
