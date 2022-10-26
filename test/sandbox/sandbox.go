package sandbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	vrs "github.com/solpipe/solpipe-tool/state/version"
	ppl "github.com/solpipe/solpipe-tool/test/people"
	sgo "github.com/SolmateDev/solana-go"
)

type Sandbox struct {
	url         string
	baseDir     string
	version     vrs.CbaVersion
	Faucet      *sgo.PrivateKey
	MintDb      *MintDb // map name to mint
	ValidatorDb *ValidatorDb
	StakerDb    *StakerDb
	People      *ppl.Group
}

func LoadFromDirectory(fp string) (*Sandbox, error) {
	e1 := new(Sandbox)
	e1.version = vrs.VERSION_1
	e1.baseDir = fp

	var err error
	e1.StakerDb, err = LoadStakersFromFile(e1.fp_staker())
	if err != nil {
		return nil, err
	}
	e1.ValidatorDb, err = LoadValidatorsFromFile(e1.fp_validators())
	if err != nil {
		return nil, err
	}

	e1.Faucet = new(sgo.PrivateKey)
	*e1.Faucet, err = sgo.PrivateKeyFromSolanaKeygenFile(e1.fp_faucet())
	if err != nil {
		return nil, err
	}

	err = e1.loadPeople()
	if err != nil {
		return nil, err
	}

	err = e1.loadMint()
	if err != nil {
		return nil, err
	}

	return e1, nil
}

func (e1 *Sandbox) WorkingDir() string {
	return e1.baseDir
}

// call this when working with solana-test-validator
func CreateForTestValidator(ledgerDir string, stakerCount int, peopleCount uint64) (*Sandbox, error) {
	e1 := new(Sandbox)
	e1.version = vrs.VERSION_1
	e1.baseDir = ""
	e1.url = "localhost"
	var err error
	err = e1.tmpDir()
	if err != nil {
		return nil, err
	}
	err = e1.createStakers(stakerCount)
	if err != nil {
		return nil, err
	}
	err = e1.loadValidatorFromTestLedger(ledgerDir)
	if err != nil {
		return nil, err
	}

	e1.People, err = ppl.NewRandomGroup(peopleCount)
	if err != nil {
		return nil, err
	}

	e1.newMint()

	err = e1.Save(e1.baseDir)
	if err != nil {
		return nil, err
	}

	return e1, nil
}

func (e1 *Sandbox) Save(targetDir string) error {
	var err error

	if e1.Faucet == nil {
		return errors.New("no faucet")
	}
	err = SavePrivateKey(*e1.Faucet, e1.fp_faucet())
	if err != nil {
		return err
	}

	err = e1.StakerDb.Save(e1.fp_staker())
	if err != nil {
		return err
	}
	err = e1.ValidatorDb.Save(e1.fp_validators())
	if err != nil {
		return err
	}

	err = e1.People.Save(e1.fp_people())
	if err != nil {
		return err
	}

	err = e1.MintDb.Save(e1.fp_mint())
	if err != nil {
		return err
	}

	if e1.baseDir != targetDir {
		err = os.Rename(e1.baseDir, targetDir)
		if err != nil {
			return err
		}
		e1.baseDir = targetDir
	}
	return nil
}

func (e1 *Sandbox) fp_faucet() string {
	return e1.baseDir + "/faucet.json"
}

func fileExists(fp string) bool {
	if _, err := os.Stat(fp); err == nil {
		return true
	} else {
		return false
	}
}

func open(fp string) (*os.File, error) {
	return os.OpenFile(fp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

func (e1 *Sandbox) tmpDir() error {
	var err error
	e1.baseDir, err = ioutil.TempDir(os.TempDir(), "solana-validator")
	if err != nil {
		return err
	}
	return nil
}

func PrivateKeyToString(key sgo.PrivateKey) string {
	d := []byte(key)
	list := make([]string, len(d))
	for i := 0; i < len(list); i++ {
		list[i] = fmt.Sprintf("%d", d[i])
	}
	return fmt.Sprintf("[%s]", strings.Join(list, ","))
}

func SavePrivateKey(key sgo.PrivateKey, fp string) error {
	f, err := open(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	ans := bytes.NewBufferString(PrivateKeyToString(key))
	_, err = io.Copy(f, ans)
	if err != nil {
		return err
	}
	return nil
}

func tmpFile(data string) (name string, err error) {
	var x *os.File
	x, err = ioutil.TempFile(os.TempDir(), "key-*")
	if err != nil {
		return
	}
	err = x.Chmod(0600)
	if err != nil {
		return
	}
	_, err = x.WriteString(data)
	if err != nil {
		return
	}
	name = x.Name()
	return
}
