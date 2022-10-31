package main

import (
	"errors"
	"strconv"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/state"
)

type ConfigRate state.Rate

type Validator struct {
	Payer    string `name:"payer" help:"The private key that owns the SOL to pay the transaction fees."`
	VoteKey  string `name:"vote" help:"The vote account for the validator."`
	StakeKey string `name:"stake" help:"Pick one stake account as a sample."`
	AdminKey string `name:"admin" help:"The admin key used to administrate the validator pipeline."`
}

func (rate *ConfigRate) UnmarshalText(data []byte) error {

	x := strings.Split(string(data), "/")
	if len(x) != 2 {
		return errors.New("rate should be if form uint64/uint64")
	}
	num, err := strconv.ParseUint(x[0], 10, 8*8)
	if err != nil {
		return err
	}
	den, err := strconv.ParseUint(x[1], 10, 8*8)
	if err != nil {
		return err
	}
	rate.N = num
	rate.D = den
	return nil
}

func (r *Validator) Run(kongCtx *CLIContext) error {
	//Create(ctx context.Context, config *Configuration, rpcClient *sgorpc.Client, wsClient *sgows.Client)
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}

	payer, err := sgo.PrivateKeyFromBase58(r.Payer)
	if err != nil {
		payer, err = sgo.PrivateKeyFromSolanaKeygenFile(r.Payer)
		if err != nil {
			return err
		}
	}

	vote, err := sgo.PrivateKeyFromBase58(r.VoteKey)
	if err != nil {
		vote, err = sgo.PrivateKeyFromSolanaKeygenFile(r.VoteKey)
		if err != nil {
			return err
		}
	}
	stake, err := sgo.PublicKeyFromBase58(r.StakeKey)
	if err != nil {
		return err
	}

	admin, err := sgo.PrivateKeyFromBase58(r.AdminKey)
	if err != nil {
		admin, err = sgo.PrivateKeyFromSolanaKeygenFile(r.AdminKey)
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
		"tcp://localhost:44332",
		nil,
	)

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}
	s1, err := relayConfig.ScriptBuilder(ctx)
	if err != nil {
		return err
	}
	err = s1.SetTx(payer)
	if err != nil {
		return err
	}

	_, err = s1.AddValidator(router.Controller.Id(), vote, stake, admin)
	if err != nil {
		return err
	}
	return s1.FinishTx(true)
}
