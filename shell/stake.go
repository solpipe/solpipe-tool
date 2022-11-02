package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	sgo "github.com/SolmateDev/solana-go"
)

type CmdResult struct {
	Signature string `json:"signature"`
}

func (s Solana) CreateStake(
	ctx context.Context,
	source sgo.PrivateKey,
	admin sgo.PrivateKey,
	amount uint64,
) (stakeId sgo.PublicKey, sig sgo.Signature, err error) {
	stake, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return
	}
	stakeId = stake.PublicKey()
	sig, err = s.CreateStakeDirect(
		ctx, source, stake, admin, amount,
	)
	return
}

func (s Solana) CreateStakeDirect(
	ctx context.Context,
	source sgo.PrivateKey,
	stakeId sgo.PrivateKey,
	admin sgo.PrivateKey,
	amount uint64,
) (sig sgo.Signature, err error) {
	sourceFp, err := tmpFile(privateKeyToString(source))
	if err != nil {
		return
	}
	defer os.Remove(sourceFp)
	stakeFp, err := tmpFile(privateKeyToString(stakeId))
	if err != nil {
		return
	}
	defer os.Remove(stakeFp)
	a := float64(amount) / float64(sgo.LAMPORTS_PER_SOL)
	cmd := exec.CommandContext(ctx,
		s.bin,
		"-u", s.rpcUrl,
		"create-stake-account",
		"--from", sourceFp,
		"--stake-authority", admin.PublicKey().String(),
		"--withdraw-authority", admin.PublicKey().String(),
		"--fee-payer", sourceFp,
		"--output", "json",
		stakeFp,
		fmt.Sprintf("%f", a),
	)
	result := new(CmdResult)
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = os.Stderr
	cmdErrorC := make(chan error, 1)
	go func() {
		cmdErrorC <- json.NewDecoder(reader).Decode(result)
	}()
	err = cmd.Run()
	if err != nil {
		return
	}
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case err = <-cmdErrorC:
	}
	if err != nil {
		return
	}
	sig, err = sgo.SignatureFromBase58(result.Signature)
	if err != nil {
		return
	}
	return
}

func (s Solana) DelegateStake(
	ctx context.Context,
	payer sgo.PrivateKey,
	stakeId sgo.PublicKey,
	admin sgo.PrivateKey,
	vote sgo.PublicKey,
) (sig sgo.Signature, err error) {
	payerFp, err := tmpFile(privateKeyToString(payer))
	if err != nil {
		return
	}
	defer os.Remove(payerFp)

	stakeAdminFp, err := tmpFile(privateKeyToString(admin))
	if err != nil {
		return
	}
	defer os.Remove(stakeAdminFp)
	cmd := exec.CommandContext(
		ctx,
		s.bin,
		"-k", payerFp,
		"-u", s.rpcUrl,
		"delegate-stake",
		"--fee-payer", payerFp,
		"--stake-authority", stakeAdminFp,
		"--output", "json",
		stakeId.String(), vote.String(),
	)
	result := new(CmdResult)
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = os.Stderr
	cmdErrorC := make(chan error, 1)
	go func() {
		cmdErrorC <- json.NewDecoder(reader).Decode(result)
	}()
	err = cmd.Run()
	if err != nil {
		return
	}
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case err = <-cmdErrorC:
	}
	if err != nil {
		return
	}
	sig, err = sgo.SignatureFromBase58(result.Signature)
	if err != nil {
		return
	}
	return
}
