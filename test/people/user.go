package people

import (
	"github.com/solpipe/solpipe-tool/script"
	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
)

type User struct {
	Id     uint64                     `json:"id"`
	Key    sgo.PrivateKey             `json:"key"`
	Wallet map[string]*sgotkn.Account `json:"wallet"` // map mint -> account
}

func (u1 *User) Account() sgo.PublicKey {
	return u1.Key.PublicKey()
}

type TokenAccount struct {
	Id      sgo.PublicKey   `json:"id"`
	Account *sgotkn.Account `json:"account"`
}

func NewRandomUser(i uint64) (*User, error) {
	var err error
	u1 := new(User)
	u1.Id = i
	u1.Key, err = sgo.NewRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	u1.Wallet = make(map[string]*sgotkn.Account)
	return u1, nil
}

func (u1 *User) Transfer(s1 *script.Script, funding sgo.PrivateKey, amount uint64) error {
	return s1.Transfer(funding, u1.Account(), amount)
}

// create a token account; make sure to use rpc to load in the token account contents into u1.Wallet (see AddTokenAccount)
func (u1 *User) InitializeTokenAccount(s1 *script.Script, mint sgo.PublicKey) error {
	_, present := u1.Wallet[mint.String()]
	if present {
		return nil
	}
	return s1.CreateTokenAccount(u1.Key, u1.Account(), mint)
}

func (u1 *User) AddTokenAccount(a *sgotkn.Account) {
	u1.Wallet[a.Mint.String()] = a
}
