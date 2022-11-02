package shell

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
)

type Solana struct {
	bin    string
	rpcUrl string
}

func CreateSolana(rpcUrl string) (s Solana, err error) {
	path, err := exec.LookPath("solana")
	if err != nil {
		return
	}
	s = Solana{
		bin:    path,
		rpcUrl: rpcUrl,
	}
	return
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

func privateKeyToString(key sgo.PrivateKey) string {
	d := []byte(key)
	list := make([]string, len(d))
	for i := 0; i < len(list); i++ {
		list[i] = fmt.Sprintf("%d", d[i])
	}
	return fmt.Sprintf("[%s]", strings.Join(list, ","))
}
