package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
)

type ValidatorDb struct {
	Validators map[string]*Validator `json:"validators"` // map id to validator
}

func LoadValidatorsFromFile(fp string) (*ValidatorDb, error) {
	db := new(ValidatorDb)
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

func (v *ValidatorDb) Save(fp string) error {
	f, err := open(fp)
	if err != nil {
		return err
	}
	return json.NewEncoder(f).Encode(v)
}

type Validator struct {
	Id    sgo.PrivateKey `json:"id"`
	Admin sgo.PrivateKey `json:"admin"`
	Vote  sgo.PrivateKey `json:"vote"`
	Port  int            `json:"port"`
}

func (e1 *Sandbox) fp_validators() string {
	return e1.baseDir + "/validators.json"
}

// loads the validator id, vote accounts, as well as the faucet account
func (e1 *Sandbox) loadValidatorFromTestLedger(ledgerDir string) error {
	var err error
	e1.ValidatorDb = new(ValidatorDb)
	e1.ValidatorDb.Validators = make(map[string]*Validator)

	v := new(Validator)
	v.Id, err = sgo.PrivateKeyFromSolanaKeygenFile(ledgerDir + "/validator-keypair.json")
	if err != nil {
		return err
	}
	v.Vote, err = sgo.PrivateKeyFromSolanaKeygenFile(ledgerDir + "/vote-account-keypair.json")
	if err != nil {
		return err
	}
	v.Admin, err = sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}
	e1.ValidatorDb.Validators[v.Id.PublicKey().String()] = v
	log.Debugf("test validator (%s;%s)", v.Id.PublicKey().String())

	e1.Faucet = new(sgo.PrivateKey)
	*e1.Faucet, err = sgo.PrivateKeyFromSolanaKeygenFile(ledgerDir + "/faucet-keypair.json")
	if err != nil {
		return err
	}

	return nil
}

// move SOL
func (e1 *Sandbox) Transfer(ctx context.Context, destination sgo.PublicKey, amount uint64) error {
	a := float64(amount) / float64(sgo.LAMPORTS_PER_SOL)
	cmd := exec.CommandContext(ctx, "solana", "-u", e1.url, "-k", e1.fp_faucet(), "transfer", "--output", "json", "--allow-unfunded-recipient", destination.String(), fmt.Sprintf("%f", a))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (e1 *Sandbox) CreateVote(ctx context.Context, id sgo.PrivateKey, vote sgo.PrivateKey, admin sgo.PublicKey) error {

	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	idFp, err := tmpFile(PrivateKeyToString(id))
	if err != nil {
		return err
	}
	go loopDelete(ctx2, idFp)
	voteFp, err := tmpFile(PrivateKeyToString(vote))
	if err != nil {
		return err
	}
	go loopDelete(ctx2, voteFp)

	cmd := exec.CommandContext(ctx2,
		"solana", "-u", e1.url,
		"create-vote-account",
		"--fee-payer", e1.fp_faucet(),
		voteFp,
		idFp,
		admin.String(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (e1 *Sandbox) AddValidator(ctx context.Context, wg *sync.WaitGroup, count int) error {
	// e1.ValidatorDb.Validators

	port := 48899

	for i := 0; i < count; i++ {
		// id and vote accounts will not be created without initial funds
		admin, err := sgo.NewRandomPrivateKey()
		if err != nil {
			return err
		}
		err = e1.Transfer(ctx, admin.PublicKey(), 30*sgo.LAMPORTS_PER_SOL)

		id, err := sgo.NewRandomPrivateKey()
		if err != nil {
			return err
		}
		err = e1.Transfer(ctx, id.PublicKey(), 30*sgo.LAMPORTS_PER_SOL)
		if err != nil {
			return err
		}

		vote, err := sgo.NewRandomPrivateKey()
		if err != nil {
			return err
		}
		err = e1.CreateVote(ctx, id, vote, admin.PublicKey())
		if err != nil {
			return err
		}
		err = e1.Transfer(ctx, vote.PublicKey(), 30*sgo.LAMPORTS_PER_SOL)
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}
		err = RunParticipantTestValidator(ctx, wg, id, vote, port)
		if err != nil {
			return err
		}
		port += 2
		e1.ValidatorDb.Validators[id.PublicKey().String()] = &Validator{
			Id: id, Vote: vote, Admin: admin, Port: port,
		}
	}
	return e1.Save(e1.baseDir)
}

// start up another solana-validator process and have it connect to the solana-test-validator process
func RunParticipantTestValidator(ctx context.Context, wg *sync.WaitGroup, id sgo.PrivateKey, vote sgo.PrivateKey, port int) error {
	log.Debugf("+++++++starting validator with id=%s and vote=%s", id.PublicKey().String(), vote.PublicKey().String())
	// solana-validator --entrypoint 127.0.0.1:8001 --log -  --tpu-use-quic   -i ./id.json --vote-account ./vote.json
	ledgerDir, err := ioutil.TempDir(os.TempDir(), "stv-*")
	if err != nil {
		return err
	}
	idFp, err := tmpFile(PrivateKeyToString(id))
	if err != nil {
		return err
	}
	go loopDelete(ctx, idFp)

	voteFp, err := tmpFile(PrivateKeyToString(vote))
	if err != nil {
		return err
	}
	go loopDelete(ctx, voteFp)

	f, err := os.Open("/dev/null")
	if err != nil {
		return err
	}
	// "--log", "-",
	cmd := exec.CommandContext(ctx,
		"solana-validator",
		"-l", ledgerDir,
		"--log", "-",
		"--private-rpc",
		"--full-rpc-api",
		"--rpc-port", fmt.Sprintf("%d", port),
		"--entrypoint", "127.0.0.1:8001",
		"--tpu-use-quic",
		"-i", idFp,
		"--vote-account", voteFp,
	)
	cmd.Stdout = f
	cmd.Stderr = f
	err = cmd.Start()
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	wg.Add(1)
	go loopValidatorWait(wg, cmd, ledgerDir)
	return nil
}

func loopDelete(ctx context.Context, fp string) {
	<-ctx.Done()
	os.Remove(fp)
}
