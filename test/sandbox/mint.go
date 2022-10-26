package sandbox

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/solpipe/solpipe-tool/script"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type MintDb struct {
	Mints map[string]*script.MintResult `json:"mints"` // map ticker to mint
}

func (db *MintDb) List() []sgo.PublicKey {
	l := make([]sgo.PublicKey, len(db.Mints))
	i := 0
	for _, k := range db.Mints {
		l[i] = *k.Id
		i++
	}
	return l
}

const (
	MINT_USD = "USD"
	MINT_EUR = "EUR"
	MINT_JPY = "JPY"
	MINT_BTC = "BTC"
	MINT_ETH = "ETH"
	MINT_SOL = "SOL"
)

func defaultMint() map[string]uint8 {
	m := make(map[string]uint8)
	m[MINT_USD] = 2
	m[MINT_BTC] = 8
	m[MINT_SOL] = 10
	return m
}

func (m *MintDb) Default(ctx context.Context, payer sgo.PrivateKey, scriptConfig script.Configuration, rpcClient *sgorpc.Client, wsClient *sgows.Client) error {
	i := 0
	var s *script.Script
	var err error
	m.Mints = make(map[string]*script.MintResult)
	for ticker, decimals := range defaultMint() {
		if s == nil {
			cfg := scriptConfig
			s, err = script.Create(ctx, &cfg, rpcClient, wsClient)
			if err != nil {
				return err
			}
			s.SetTx(payer)
		}
		err = m.Create(payer, ticker, decimals, s)
		if err != nil {
			return err
		}

		if i < 4 {
			i++
		} else {
			i = 0
			err = s.FinishTx(false)
			if err != nil {
				return err
			}
			s = nil
		}
	}
	if s != nil {
		err = s.FinishTx(false)
		if err != nil {
			return err
		}
	}
	err = m.LoadData(ctx, rpcClient)
	if err != nil {
		return err
	}
	return nil
}

func (m *MintDb) Create(payer sgo.PrivateKey, ticker string, decimals uint8, s1 *script.Script) error {
	_, present := m.Mints[ticker]
	if present {
		return errors.New("mint already exists")
	}
	authority, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}
	result, err := s1.CreateMint(payer, authority, decimals)
	if err != nil {
		return err
	}
	m.Mints[ticker] = result
	return nil
}

// do an rpc call to download mint data
func (m *MintDb) LoadData(ctx context.Context, rpcClient *sgorpc.Client) error {

	errorC := make(chan error, 20)
	i := 0
	for _, x := range m.Mints {
		if x.Data == nil {
			i++
			go loopGetData(ctx, rpcClient, errorC, x)
		}
	}
	doneC := ctx.Done()
	var err error
out:
	for k := 0; k < i; k++ {
		select {
		case <-doneC:
			err = errors.New("canceled")
			break out
		case err = <-errorC:
			if err != nil {
				break out
			}
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func loopGetData(ctx context.Context, rpcClient *sgorpc.Client, errorC chan<- error, x *script.MintResult) {
	x.Data = new(sgotkn.Mint)
	select {
	case errorC <- rpcClient.GetAccountDataBorshInto(ctx, *x.Id, x.Data):
	}
}

func (m *MintDb) Save(fp string) error {
	f, err := open(fp)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(m)
	if err != nil {
		return err
	}
	return nil
}

func (e1 *Sandbox) fp_mint() string {
	return e1.baseDir + "/pc-mint.json"
}

func (e1 *Sandbox) newMint() {
	db := new(MintDb)
	db.Mints = make(map[string]*script.MintResult)
	e1.MintDb = db
}

func (e1 *Sandbox) loadMint() error {
	f, err := open(e1.fp_mint())
	e1.MintDb = new(MintDb)
	err = json.NewDecoder(f).Decode(e1.MintDb)
	if err != nil {
		return err
	}
	return nil
}
