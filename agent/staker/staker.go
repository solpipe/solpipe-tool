// monitor the validator we stake on to make sure our stake is counted in payouts
package staker

import (
	"context"
	"errors"
	"os"

	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type Staker struct {
	ctx        context.Context
	cancel     context.CancelFunc
	internalC  chan<- func(*internal)
	controller ctr.Controller
	router     rtr.Router
	staker     skr.Staker
}

type Configuration struct {
	Admin sgo.PrivateKey
	Stake sgo.PublicKey // public key
}

func ConfigFromEnv() (*Configuration, error) {
	ans := new(Configuration)
	var err error
	ans.Admin, err = sgo.PrivateKeyFromBase58(os.Getenv("STAKER_ADMIN"))
	if err != nil {
		ans.Admin, err = sgo.PrivateKeyFromSolanaKeygenFile(os.Getenv("STAKER_ADMIN"))
		if err != nil {
			return nil, err
		}
	}
	ans.Stake, err = sgo.PublicKeyFromBase58(os.Getenv("STAKER_STAKE"))
	if err != nil {
		return nil, err
	}
	return ans, nil
}

func Create(
	ctxOutside context.Context,
	config *Configuration,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	router rtr.Router,
) (Staker, error) {
	var err error
	ctx, cancel := context.WithCancel(ctxOutside)
	internalC := make(chan func(*internal), 10)
	if config == nil {
		config, err = ConfigFromEnv()
		if err != nil {
			cancel()
			return Staker{}, err
		}
	}
	sub, err := wsClient.AccountSubscribe(config.Stake, sgorpc.CommitmentFinalized)
	if err != nil {
		cancel()
		return Staker{}, err
	}

	go loopInternal(ctx, internalC, cancel, router, config, sub)
	e1 := Staker{
		ctx: ctx, cancel: cancel, internalC: internalC, router: router,
	}
	return e1, nil
}

func loopStakeAccount(ctx context.Context, errorC chan<- error, sub *sgows.AccountSubscription) {
	defer sub.Unsubscribe()
	var err error

	streamC := sub.RecvStream()
	finishC := sub.RecvErr()
	doneC := ctx.Done()

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-finishC:
			break out
		case <-streamC:
			// we cannot deserialize staking accounts, so ...
		}
	}
	if err != nil {
		errorC <- err
	}
}

func (e1 Staker) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("already done")
		signalC <- err
		return signalC
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}

	return signalC
}

func (e1 Staker) Close() {
	doneC := e1.CloseSignal()
	e1.cancel()
	<-doneC
}
