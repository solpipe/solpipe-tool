package people

import (
	"context"
	"errors"

	"github.com/solpipe/solpipe-tool/script"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

//  rpcClient *sgorpc.Client, wsClient *sgows.Client
type Preparation struct {
	Ctx        context.Context
	Mint       []sgo.PublicKey
	Faucet     sgo.PrivateKey
	RpcUrl     string
	WsUrl      string
	SeedAmount uint64
}

func (p *Preparation) Rpc() *sgorpc.Client {
	return sgorpc.New(p.RpcUrl)
}

func (p *Preparation) Ws() (*sgows.Client, error) {
	return sgows.Connect(p.Ctx, p.WsUrl)
}

func (g *Group) Prepare(p *Preparation) error {
	errorC := make(chan error, 10)
	doneC := p.Ctx.Done()
	ctx2, cancel := context.WithCancel(p.Ctx)
	defer cancel()
	for _, user := range g.Users {
		{
			wsClient, err := p.Ws()
			if err != nil {
				log.Debug("w - 1")
				log.Debug(err)
				return err
			}
			mintC := make(chan sgo.PublicKey, len(p.Mint))

			go loopRunUser(errorC, ctx2, wsClient, p.Rpc(), p.Faucet, user, p.SeedAmount, mintC, len(p.Mint))
			for _, m := range p.Mint {
				mintC <- m
			}
		}
	}
	var err error
	for i := 0; i < len(g.Users); i++ {
		select {
		case <-doneC:
			return errors.New("canceled")
		case err = <-errorC:
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func loopRunUser(errorC chan<- error, ctx context.Context, wsClient *sgows.Client, rpcClient *sgorpc.Client, faucet sgo.PrivateKey, user *User, seedAmount uint64, mintC <-chan sgo.PublicKey, mintCount int) {
	errorC <- insideLoopRunUser(ctx, wsClient, rpcClient, faucet, user, seedAmount, mintC, mintCount)
	// do other stuff?  like pass money around
}

func insideLoopRunUser(ctx context.Context, wsClient *sgows.Client, rpcClient *sgorpc.Client, faucet sgo.PrivateKey, user *User, seedAmount uint64, mintC <-chan sgo.PublicKey, mintCount int) error {
	defer wsClient.Close()
	s1, err := script.Create(ctx, &script.Configuration{Version: vrs.VERSION_1}, rpcClient, wsClient)
	if err != nil {
		return err
	}
	err = s1.SetTx(faucet)
	if err != nil {
		log.Debug("s - 1")
		log.Debug(err)
		return err
	}
	err = s1.Transfer(faucet, user.Account(), seedAmount)
	if err != nil {
		log.Debug("s - 2")
		log.Debug(err)
		return err
	}
	fetchMint := make([]sgo.PublicKey, mintCount)
	doneC := ctx.Done()
out:
	for i := 0; i < mintCount; i++ {
		select {
		case <-doneC:
			break out
		case m := <-mintC:
			err = user.InitializeTokenAccount(s1, m)
			if err != nil {
				log.Debug("s - 3")
				log.Debug(err)
				return err
			}
			fetchMint[i] = m
		}
	}
	err = s1.FinishTx(true)
	if err != nil {
		log.Debug("s - 4 - %s", err)
		return err
	}
	for i := 0; i < len(fetchMint); i++ {
		a, _, err := sgo.FindAssociatedTokenAddress(user.Account(), fetchMint[i])
		if err != nil {
			log.Debug("s - 5")
			log.Debug(err)
			return err
		}
		account := new(sgotkn.Account)
		err = rpcClient.GetAccountDataBorshInto(ctx, a, account)
		if err != nil {
			log.Debug("s - 6")
			log.Debug(err)
			return err
		}
		log.Debugf("adding user token for mint=%s", fetchMint[i].String())
		user.AddTokenAccount(account)
	}
	return nil
}
