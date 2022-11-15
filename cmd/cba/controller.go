package main

import (
	"errors"
	"os"
	"strconv"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/state"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
)

type Controller struct {
	Create ControllerCreate `cmd name:"create" help:"Create a controller and change the controller settings"`
	Status ControllerStatus `cmd name:"status" help:"Print the admin, token balance of the controller"`
}

type ControllerStatus struct {
	Admin string `arg name:"admin" help:"the file path of the private key"`
}

func (r *ControllerStatus) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx

	admin, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Admin)
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
	version := relayConfig.Version
	controller, err := ctr.CreateController(ctx, rpcClient, wsClient, version)
	if err != nil {
		return err
	}

	ans, err := controller.Print()
	if err != nil {
		return err
	}

	os.Stdout.WriteString(ans)

	return nil
}

type ControllerCreate struct {
	Payer   string `name:"payer" short:"p" help:"the account paying SOL fees"`
	Admin   string `arg name:"admin" short:"a" help:"the account with administrative privileges"`
	Cranker string `option name:"cranker" help:"who is allowed to crank"`
	Mint    string `arg name:"mint" short:"m" help:"the mint of the token account to which fees are paid to and by validators"`
	Fee     string `arg name:"fee" short:"f" help:"set the fee that the controller earns from Validator revenue."`
}

func (r *ControllerCreate) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx

	payer, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Payer)
	if err != nil {
		return err
	}
	admin, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Admin)
	if err != nil {
		return err
	}
	mint, err := sgo.PublicKeyFromBase58(r.Mint)
	if err != nil {
		return err
	}
	fee := new(state.Rate)
	{
		x := strings.Split(r.Fee, "/")
		if len(x) != 2 {
			return errors.New("fee is not of form numerator/denominator")
		}
		fee.N, err = strconv.ParseUint(x[0], 10, 64)
		if err != nil {
			return err
		}
		fee.D, err = strconv.ParseUint(x[1], 10, 64)
		if err != nil {
			return err
		}
	}

	var cranker sgo.PrivateKey
	if len(r.Cranker) == 0 {
		cranker = admin
	} else {
		cranker, err = sgo.PrivateKeyFromSolanaKeygenFile(r.Cranker)
		if err != nil {
			return err
		}
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
	s1, err := relayConfig.ScriptBuilder(ctx)
	if err != nil {
		return err
	}
	err = s1.SetTx(payer)
	if err != nil {
		return err
	}
	err = s1.CreateController(admin, cranker, mint, fee)
	if err != nil {
		return err
	}
	err = s1.FinishTx(true)
	if err != nil {
		return err
	}

	return nil
}
