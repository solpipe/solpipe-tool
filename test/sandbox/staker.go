package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type StakerDb struct {
	Stakers []*Staker
}

func (e1 *Sandbox) fp_staker() string {
	return e1.baseDir + "/stakers.json"
}

func LoadStakersFromFile(fp string) (*StakerDb, error) {
	db := new(StakerDb)
	f, err := open(fp)
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(f).Decode(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (db *StakerDb) Save(fp string) error {
	f, err := open(fp)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(db)
}

type Staker struct {
	Admin sgo.PrivateKey
	Stake sgo.PrivateKey
}

func (e1 *Sandbox) createStakers(count int) error {
	var err error
	e1.StakerDb = new(StakerDb)
	e1.StakerDb.Stakers = make([]*Staker, count)
	for i := 0; i < count; i++ {

		e1.StakerDb.Stakers[i] = new(Staker)
		e1.StakerDb.Stakers[i].Admin, err = sgo.NewRandomPrivateKey()
		if err != nil {
			return err
		}
		e1.StakerDb.Stakers[i].Stake, err = sgo.NewRandomPrivateKey()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e1 *Sandbox) CreateStake(ctx context.Context, i int, amount uint64) error {
	if i < 0 || len(e1.StakerDb.Stakers) <= i {
		return errors.New("i out of bound")
	}

	faucetFp, err := tmpFile(PrivateKeyToString(*e1.Faucet))
	if err != nil {
		return err
	}
	defer os.Remove(faucetFp)

	stakeFp, err := tmpFile(PrivateKeyToString(e1.StakerDb.Stakers[i].Stake))
	if err != nil {
		return err
	}
	defer os.Remove(stakeFp)

	// solana -k ./local-test.json -u https://rpc.test.solmate.dev create-stake-account ./local-stake.json 200
	a := float64(amount) / float64(sgo.LAMPORTS_PER_SOL)
	err = exec.CommandContext(ctx, "solana", "-k", faucetFp, "-u", e1.url, "create-stake-account", stakeFp, fmt.Sprintf("%f", a)).Run()
	if err != nil {
		return err
	}

	return nil
}

func (e1 *Sandbox) DelegateStake(ctx context.Context, i int, vote sgo.PublicKey) error {
	if i < 0 || len(e1.StakerDb.Stakers) <= i {
		return errors.New("i out of bound")
	}
	faucetFp, err := tmpFile(PrivateKeyToString(*e1.Faucet))
	if err != nil {
		return err
	}
	defer os.Remove(faucetFp)

	stakeFp, err := tmpFile(PrivateKeyToString(e1.StakerDb.Stakers[i].Stake))
	if err != nil {
		return err
	}
	defer os.Remove(stakeFp)
	// solana -k ./local-test.json -u https://rpc.test.solmate.dev delegate-stake ./local-stake.json $(solana-keygen pubkey ./sandbox/node-0/vote.json)
	err = exec.CommandContext(ctx, "solana", "-k", faucetFp, "-u", e1.url, "delegate-stake", stakeFp, vote.String()).Run()
	if err != nil {
		return err
	}
	return nil
}

type CmdResult struct {
	Signature string `json:"signature"`
}

func CreateStake(
	ctx context.Context, url string,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	fundSource sgo.PrivateKey,
	stakeAccount sgo.PrivateKey,
	admin sgo.PrivateKey,
	amount uint64,
) error {

	fundsFp, err := tmpFile(PrivateKeyToString(fundSource))
	if err != nil {
		return err
	}
	defer os.Remove(fundsFp)

	stakeFp, err := tmpFile(PrivateKeyToString(stakeAccount))
	if err != nil {
		return err
	}
	defer os.Remove(stakeFp)

	// solana -k ./local-test.json -u https://rpc.test.solmate.dev create-stake-account ./local-stake.json 200

	// solana create-stake-account --from <KEYPAIR> stake-account.json <AMOUNT> \
	// --stake-authority <KEYPAIR> --withdraw-authority <KEYPAIR> \
	// --fee-payer <KEYPAIR>
	a := float64(amount) / float64(sgo.LAMPORTS_PER_SOL)
	//cmd := exec.CommandContext(ctx, "solana", "-k", faucetFp, "-u", url, "create-stake-account", stakeFp, fmt.Sprintf("%f", a))
	log.Debugf("stake account=%s url=%s", stakeAccount.PublicKey().String(), url)
	cmd := exec.CommandContext(ctx,
		"solana", "-u", url,
		"create-stake-account",
		"--from", fundsFp,
		"--stake-authority", admin.PublicKey().String(),
		"--withdraw-authority", admin.PublicKey().String(),
		"--fee-payer", fundsFp,
		"--output", "json",
		stakeFp,
		fmt.Sprintf("%f", a),
	)
	reader, writer := io.Pipe()
	result := new(CmdResult)
	cmd.Stdout = writer
	cmd.Stderr = os.Stderr
	cmdErrorC := make(chan error, 1)
	go func() {
		cmdErrorC <- json.NewDecoder(reader).Decode(result)
	}()
	err = cmd.Run()
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case err = <-cmdErrorC:
	}
	if err != nil {
		return err
	}
	sig, err := sgo.SignatureFromBase58(result.Signature)
	if err != nil {
		return err
	}
	log.Debugf("searching for signature: %s", sig.String())
	var r *sgorpc.GetTransactionResult
out:
	for i := 0; i < 20; i++ {
		r, err = rpcClient.GetTransaction(ctx, sig, &sgorpc.GetTransactionOpts{Commitment: sgorpc.CommitmentFinalized})
		if err == nil {
			if r.Meta.Err == nil {
				log.Debugf("stake created: (%d,%d)", r.Slot, r.Meta.Fee)
				break out
			} else {
				err = fmt.Errorf("%+v", r.Meta.Err)
				log.Debug(err)
			}
		} else {
			log.Debug(err)
		}
		time.Sleep(5 * time.Second)
	}
	log.Debug("found signature")

	//err = util.WaitSig(ctx, wsClient, sig)
	if err != nil {
		return err
	}

isok:
	for i := 0; i < 10; i++ {
		cmd = exec.CommandContext(ctx, "solana", "-u", url, "stake-account", stakeAccount.PublicKey().String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err == nil {
			break isok
		}
		time.Sleep(1 * time.Minute)
	}
	if err != nil {
		return err
	}

	// solana stake-account <STAKE_ACCOUNT_ADDRESS>

	return nil
}

func DelegateStake(ctx context.Context, url string, payer sgo.PrivateKey, stake sgo.PublicKey, stakeAdmin sgo.PrivateKey, vote sgo.PublicKey) error {

	payerFp, err := tmpFile(PrivateKeyToString(payer))
	if err != nil {
		return err
	}
	defer os.Remove(payerFp)

	stakeAdminFp, err := tmpFile(PrivateKeyToString(stakeAdmin))
	if err != nil {
		return err
	}
	defer os.Remove(stakeAdminFp)
	// solana -k ./local-test.json -u https://rpc.test.solmate.dev delegate-stake ./local-stake.json $(solana-keygen pubkey ./sandbox/node-0/vote.json)
	{
		cmd2 := exec.CommandContext(ctx,
			"solana", "-u", url,
			"stake-account",
			"--output", "json",
			stake.String(),
		)
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		cmd2.Run()
	}
	cmd := exec.CommandContext(ctx,
		"solana", "-u", url,
		"delegate-stake",
		"--fee-payer", payerFp,
		"--stake-authority", stakeAdminFp,
		"--output", "json",
		stake.String(),
		vote.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	{
		cmd2 := exec.CommandContext(ctx,
			"solana", "-u", url,
			"stake-account",
			"--output", "json",
			stake.String(),
		)
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		cmd2.Run()
	}
	return nil
}
