package sandbox

import (
	"context"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// deploy a program to the validator
func (e1 *Sandbox) Deploy(ctx context.Context, programKeyPairFilePath string, bpfFilePath string) error {
	// solana --keypair $VALIDATOR_FP --url localhost program deploy --program-id $filepath ./target/deploy/solmate_cba.so
	wallet, err := tmpFile(PrivateKeyToString(*e1.Faucet))
	if err != nil {
		return err
	}
	defer os.Remove(wallet)
	log.Debug("deploying Solana programs")
	cmd := exec.CommandContext(ctx, "solana", "-k", wallet, "-u", e1.url, "program", "deploy", "--program-id", programKeyPairFilePath, bpfFilePath)
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
