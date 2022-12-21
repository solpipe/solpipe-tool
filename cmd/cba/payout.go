package main

import (
	"encoding/json"
	"errors"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
)

type Payout struct {
	Status PayoutStatus `cmd name:"status" help:"create a pipeline"`
}

type PayoutStatus struct {
	Validator string `arg name:"validator"  help:"public key of validator member account"`
	Payout    string `arg name:"payout"  help:"public key of payout"`
}

func (r *PayoutStatus) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}
	rpcClient := sgorpc.New(kongCtx.Clients.RpcUrl)
	summary := new(ReceiptSummary)
	var err error
	{
		log.Debug("p - 1")
		payoutId, err := sgo.PublicKeyFromBase58(r.Payout)
		if err != nil {
			return err
		}
		log.Debug("p - 2")
		summary.Payout = new(cba.Payout)
		err = rpcClient.GetAccountDataInto(ctx, payoutId, summary.Payout)
		if err != nil {
			return err
		}
	}
	{
		log.Debug("p - 3")
		summary.Validator = new(cba.ValidatorManager)
		validatorId, err := sgo.PublicKeyFromBase58(r.Validator)
		if err != nil {
			return err
		}
		err = rpcClient.GetAccountDataBorshInto(ctx, validatorId, summary.Validator)
		if err != nil {
			return err
		}
		log.Debug("p - 4")
		list := make([]*cba.Receipt, len(summary.Validator.Ring))
		i := 0
		for _, r := range summary.Validator.Ring {

			if 0 < r.Start {
				log.Debugf("r=%+v", r)
				list[i] = new(cba.Receipt)
				err = rpcClient.GetAccountDataInto(ctx, r.Receipt, list[i])
				if err != nil {
					return err
				}
				i++
			} else {
				list[i] = nil
			}

		}
		log.Debug("p - 5")
		summary.ReceiptList = list

	}
	err = json.NewEncoder(os.Stderr).Encode(summary)
	if err != nil {
		return err
	}
	os.Stdout.WriteString("\n\n")
	return nil
}

type ReceiptSummary struct {
	Payout      *cba.Payout
	Validator   *cba.ValidatorManager
	ReceiptList []*cba.Receipt
}
