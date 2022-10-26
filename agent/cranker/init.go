package cranker

import (
	"context"

	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
)

func SetupPcVault(
	ctx context.Context,
	router rtr.Router,
	config *relay.Configuration,
) (*sgotkn.Account, error) {
	controllerData, err := router.Controller.Data()
	if err != nil {
		return nil, err
	}
	id, err := pcVaultId(&sgotkn.Account{
		Mint:  controllerData.PcMint,
		Owner: config.Admin.PublicKey(),
	})
	if err != nil {
		return nil, err
	}
	account := new(sgotkn.Account)
	err = config.Rpc().GetAccountDataBorshInto(ctx, id, account)
	if err != nil {
		script, err := config.ScriptBuilder(ctx)
		if err != nil {
			return nil, err
		}
		err = createPcVault(ctx, config, controllerData.PcMint, script)
		if err != nil {
			return nil, err
		}
		err = config.Rpc().GetAccountDataBorshInto(ctx, id, account)
		if err != nil {
			return nil, err
		}
	}

	return account, nil
}

func createPcVault(
	ctx context.Context,
	config *relay.Configuration,
	mint sgo.PublicKey,
	script *script.Script,
) error {
	var err error
	err = script.SetTx(config.Admin)
	if err != nil {
		return err
	}
	err = script.CreateTokenAccount(config.Admin, config.Admin.PublicKey(), mint)
	if err != nil {
		return err
	}
	err = script.FinishTx(false)
	if err != nil {
		return err
	}
	return nil

}
