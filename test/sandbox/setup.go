package sandbox

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type TestSetup struct {
	wg      *sync.WaitGroup
	Sandbox *Sandbox
	Rpc     *sgorpc.Client
	Ws      *sgows.Client
}

// the ledger directory automatically deletes itself
func RunTestValidator(
	ctx context.Context,
	wg *sync.WaitGroup,
	ledgerDir string,
	stakerCount int,
	peopleCount uint64,
) (*TestSetup, error) {
	var err error
	if ledgerDir == "" {
		ledgerDir, err = ioutil.TempDir(os.TempDir(), "stv-*")
		if err != nil {
			return nil, err
		}
	}
	cmd := exec.CommandContext(ctx, "solana-test-validator", "-l", ledgerDir, "--limit-ledger-size", "10000", "--rpc-pubsub-enable-vote-subscription", "--bind-address", "127.0.0.1", "--gossip-host", "127.0.0.1", "--gossip-port", "8001")
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	wg.Add(1)
	go loopValidatorWait(wg, cmd, ledgerDir)
	time.Sleep(10 * time.Second)
	rpcClient := sgorpc.New("http://localhost:8899")
	wsClient, err := sgows.Connect(ctx, "ws://localhost:8900")
	if err != nil {
		return nil, err
	}

	sb, err := CreateForTestValidator(ledgerDir, stakerCount, peopleCount)
	if err != nil {
		// kill command by canceling the context
		return nil, err
	}

	return &TestSetup{
		Sandbox: sb, Rpc: rpcClient, Ws: wsClient, wg: wg,
	}, nil
}

func loopValidatorWait(wg *sync.WaitGroup, cmd *exec.Cmd, ledgerDir string) {
	err := cmd.Wait()
	if err != nil {
		log.Debug(err)
	}
	os.RemoveAll(ledgerDir)
	wg.Done()
}
