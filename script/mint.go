package script

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn2 "github.com/SolmateDev/solana-go/programs/associated-token-account"
	sgosys "github.com/SolmateDev/solana-go/programs/system"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
)

const MINT_SIZE = 4 + 40 + 8 + 1 + 1 + 4 + 40

/*exp
ort const MintLayout = struct<RawMint>([
    u32('mintAuthorityOption'),
    publicKey('mintAuthority'),
    u64('supply'),
    u8('decimals'),
    bool('isInitialized'),
    u32('freezeAuthorityOption'),
    publicKey('freezeAuthority'),
]);*/

const TOKEN_ACCOUNT_SIZE = 40 + 40 + 8 + 4 + 40 + 1 + 4 + 8 + 8 + 4 + 40

/*
   publicKey('mint'),
   publicKey('owner'),
   u64('amount'),
   u32('delegateOption'),
   publicKey('delegate'),
   u8('state'),
   u32('isNativeOption'),
   u64('isNative'),
   u64('delegatedAmount'),
   u32('closeAuthorityOption'),
   publicKey('closeAuthority'),
*/

func LoadMint(ctx context.Context, mintId sgo.PublicKey, authority sgo.PrivateKey, rpcClient *sgorpc.Client) (*MintResult, error) {
	mr := new(MintResult)
	mr.Id = mintId.ToPointer()
	mr.Authority = &authority
	err := mr.Fill(ctx, rpcClient)
	if err != nil {
		return nil, err
	}
	return mr, nil
}

func (e1 *Script) CreateMint(payer sgo.PrivateKey, authority sgo.PrivateKey, decimals uint8) (*MintResult, error) {
	mint, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return nil, err
	}
	return e1.CreateMintDirect(mint, payer, authority, decimals)
}

func (e1 *Script) CreateMintDirect(mint sgo.PrivateKey, payer sgo.PrivateKey, authority sgo.PrivateKey, decimals uint8) (*MintResult, error) {
	if e1.txBuilder == nil {
		return nil, errors.New("tx builder is blank")
	}

	e1.AppendKey(mint)
	{
		var space uint64 = 82
		minLamports, err := e1.rpc.GetMinimumBalanceForRentExemption(e1.ctx, space, sgorpc.CommitmentConfirmed)
		if err != nil {
			return nil, err
		}
		e1.txBuilder.AddInstruction(sgosys.NewCreateAccountInstructionBuilder().SetSpace(space).SetLamports(minLamports).SetOwner(sgotkn.ProgramID).SetFundingAccount(payer.PublicKey()).SetNewAccount(mint.PublicKey()).Build())
	}
	{
		e1.txBuilder.AddInstruction(sgotkn.NewInitializeMintInstructionBuilder().SetDecimals(decimals).SetMintAccount(mint.PublicKey()).SetMintAuthority(authority.PublicKey()).SetFreezeAuthority(authority.PublicKey()).Build())
	}
	id := mint.PublicKey()
	auth := authority
	return &MintResult{
		Id:        &id,
		Data:      nil,
		Authority: &auth,
	}, nil

}

func (e1 *Script) CreateTokenAccount(payer sgo.PrivateKey, owner sgo.PublicKey, mint sgo.PublicKey) error {
	if e1.txBuilder == nil {
		return errors.New("tx builder is blank")
	}
	b := sgotkn2.NewCreateInstructionBuilder()
	b.SetMint(mint)
	b.SetPayer(payer.PublicKey())
	e1.AppendKey(payer)
	b.SetWallet(owner)
	e1.txBuilder.AddInstruction(b.Build())
	return nil
}

func (e1 *Script) MintIssue(mr *MintResult, owner sgo.PublicKey, amount uint64) error {
	if e1.txBuilder == nil {
		return errors.New("tx builder is blank")
	}
	if mr == nil {
		return errors.New("no mint result")
	}
	if mr.Id == nil {
		return errors.New("no mint id")
	}
	if mr.Authority == nil {
		return errors.New("no mint authority")
	}
	destination, _, err := sgo.FindAssociatedTokenAddress(owner, *mr.Id)
	if err != nil {
		return err
	}

	b := sgotkn.NewMintToInstructionBuilder()
	b.SetAmount(amount)
	b.SetAuthorityAccount(mr.Authority.PublicKey())
	e1.AppendKey(*mr.Authority)
	b.SetDestinationAccount(destination)
	b.SetMintAccount(*mr.Id)

	e1.txBuilder.AddInstruction(b.Build())
	return nil
}

type MintResult struct {
	Id        *sgo.PublicKey  `json:"id"`
	Data      *sgotkn.Mint    `json:"data"`
	Authority *sgo.PrivateKey `json:"authority"`
}

// download the account data defining the Mint
func (mr *MintResult) Fill(ctx context.Context, rpcClient *sgorpc.Client) error {
	if mr == nil {
		return errors.New("blank mint result")
	}
	if mr.Id == nil {
		return errors.New("no id")
	}
	mr.Data = new(sgotkn.Mint)
	return rpcClient.GetAccountDataBorshInto(ctx, *mr.Id, mr.Data)
}

// save the Mint data to disk, including the authority private key
func (mr *MintResult) Save(fp string) error {
	f, err := os.Open(fp)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(mr)
}

// load the Mint data from disk, including the authority private key
func LoadMintFromFile(fp string) (*MintResult, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	ans := new(MintResult)
	err = json.NewDecoder(f).Decode(ans)
	if err != nil {
		return nil, err
	}
	return ans, nil
}
