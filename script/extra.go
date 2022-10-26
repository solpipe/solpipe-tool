package script

import (
	"context"
	"errors"
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	sgosys "github.com/SolmateDev/solana-go/programs/system"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	bin "github.com/gagliardetto/binary"
)

func (e1 *Script) CreateAccountDirect(size uint64, account sgo.PrivateKey, owner sgo.PublicKey, payer sgo.PrivateKey) (sgo.PublicKey, error) {
	if e1.txBuilder == nil {
		return sgo.PublicKey{}, errors.New("tx builder is blank")
	}

	lamports, err := e1.rpc.GetMinimumBalanceForRentExemption(e1.ctx, size, sgorpc.CommitmentFinalized)
	if err != nil {
		return sgo.PublicKey{}, err
	}
	b := sgosys.NewCreateAccountInstructionBuilder()
	b.SetFundingAccount(payer.PublicKey())
	e1.AppendKey(payer)
	b.SetLamports(lamports)
	b.SetNewAccount(account.PublicKey())
	e1.AppendKey(account)
	b.SetOwner(owner)
	b.SetSpace(size)
	e1.txBuilder.AddInstruction(b.Build())
	return owner, nil
}

// provide space=x bytes to create an empty account
func (e1 *Script) CreateAccount(size uint64, owner sgo.PublicKey, payer sgo.PrivateKey) (sgo.PublicKey, error) {
	if e1.txBuilder == nil {
		return sgo.PublicKey{}, errors.New("tx builder is blank")
	}
	id, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return sgo.PublicKey{}, err
	}
	lamports, err := e1.rpc.GetMinimumBalanceForRentExemption(e1.ctx, size, sgorpc.CommitmentFinalized)
	if err != nil {
		return sgo.PublicKey{}, err
	}
	b := sgosys.NewCreateAccountInstructionBuilder()
	b.SetFundingAccount(payer.PublicKey())
	e1.AppendKey(payer)
	b.SetLamports(lamports)
	b.SetNewAccount(id.PublicKey())
	e1.AppendKey(id)
	b.SetOwner(owner)
	b.SetSpace(size)
	e1.txBuilder.AddInstruction(b.Build())
	return id.PublicKey(), nil
}

func Airdrop(ctx context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client, destination sgo.PublicKey, amount uint64) error {
	sig, err := rpcClient.RequestAirdrop(ctx, destination, amount, sgorpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	return WaitSig(ctx, wsClient, sig)
}

func WaitSig(ctx context.Context, wsClient *sgows.Client, sig sgo.Signature) error {

	sub, err := wsClient.SignatureSubscribe(sig, sgorpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case err = <-sub.CloseSignal():
	case d := <-sub.RecvStream():
		x, ok := d.(*sgows.SignatureResult)
		if !ok {
			err = errors.New("unknown result")
		} else {
			if x.Value.Err != nil {
				err = fmt.Errorf("%+v", x.Value.Err)
			}
		}
	}
	return err
}

func GetTokenAccount(ctx context.Context, rpcClient *sgorpc.Client, owner sgo.PublicKey, mint sgo.PublicKey) (*sgotkn.Account, error) {
	accountId, _, err := sgo.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return nil, err
	}
	a := new(sgotkn.Account)
	d, err := rpcClient.GetAccountInfo(ctx, accountId)
	if err != nil {
		return nil, err
	}
	err = bin.UnmarshalBorsh(a, d.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return a, nil
}
