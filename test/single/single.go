package single

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/script"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type SingleSandbox struct {
	Version                 vrs.CbaVersion
	Faucet                  sgo.PrivateKey
	ControllerAdmin         sgo.PrivateKey
	Mint                    sgo.PrivateKey
	MintAuthority           sgo.PrivateKey
	Validator               sgo.PrivateKey
	ValidatorAdmin          sgo.PrivateKey
	ValidatorClearNet       *relay.ClearNetListenConfig
	Vote                    sgo.PrivateKey
	Pipeline                sgo.PrivateKey // needed to get the pipeline Id via PublicKey()
	PipelineAdmin           sgo.PrivateKey
	PipelineClearNet        *relay.ClearNetListenConfig
	Bidder                  []sgo.PrivateKey
	BidderConfigFilePath    []string
	Cranker                 sgo.PrivateKey
	rpcUrl                  string
	wsUrl                   string
	pipelineAdminListenUrl  string
	validatorAdminListenUrl string
}

func (s *SingleSandbox) CrankerConfig() relay.Configuration {
	return relay.CreateConfiguration(
		s.Version,
		s.PipelineAdmin,
		s.rpcUrl,
		s.wsUrl,
		http.Header{},
		"",
		nil,
	)
}

func (s *SingleSandbox) BidderConfig(i int) (relay.Configuration, error) {
	if i < 0 || len(s.Bidder) <= i {
		return relay.Configuration{}, errors.New("i out of range")
	}
	return relay.CreateConfiguration(
		s.Version,
		s.Bidder[i],
		s.rpcUrl,
		s.wsUrl,
		http.Header{},
		"",
		nil,
	), nil
}

func (s *SingleSandbox) PipelineConfig() relay.Configuration {
	return relay.CreateConfiguration(
		s.Version,
		s.PipelineAdmin,
		s.rpcUrl,
		s.wsUrl,
		http.Header{},
		s.pipelineAdminListenUrl,
		s.PipelineClearNet,
	)
}
func (s *SingleSandbox) ValidatorConfig() relay.Configuration {
	return relay.CreateConfiguration(
		s.Version,
		s.ValidatorAdmin,
		s.rpcUrl,
		s.wsUrl,
		http.Header{},
		s.validatorAdminListenUrl,
		s.ValidatorClearNet,
	)
}

func (s *SingleSandbox) Script(ctx context.Context) (*script.Script, error) {
	pc := s.PipelineConfig()
	rpcClient := pc.Rpc()
	wsClient, err := pc.Ws(ctx)
	if err != nil {
		return nil, err
	}
	return script.Create(ctx, &script.Configuration{Version: s.Version}, rpcClient, wsClient)
}

// ctx is used to clean up sockets after testing has finished
// deploy program with   anchor deploy --program-keypair ./localconfig/single/program.json --program-name solmate_cba --provider.wallet ./localconfig/single/faucet.json
func Load(ctx context.Context) (*SingleSandbox, error) {
	fp, present := os.LookupEnv("SANDBOX_SINGLE")
	if !present {
		return nil, errors.New("SANDBOX_SINGLE env var not set")
	}
	var err error
	ans := new(SingleSandbox)
	ans.pipelineAdminListenUrl = "unix:///tmp/pipeline.socket"
	go loopDelete(ctx, ans.pipelineAdminListenUrl)
	ans.validatorAdminListenUrl = "unix:///tmp/validator.socket"
	go loopDelete(ctx, ans.validatorAdminListenUrl)
	ans.Faucet, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/faucet.json")
	if err != nil {
		return nil, err
	}
	ans.ControllerAdmin, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/controller-admin.json")
	if err != nil {
		return nil, err
	}
	ans.Mint, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/mint.json")
	if err != nil {
		return nil, err
	}
	ans.MintAuthority, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/mint-authority.json")
	if err != nil {
		return nil, err
	}
	ans.Validator, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/validator.json")
	if err != nil {
		return nil, err
	}
	ans.ValidatorAdmin, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/validator-admin.json")
	if err != nil {
		return nil, err
	}
	ans.Vote, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/vote.json")
	if err != nil {
		return nil, err
	}

	ans.Pipeline, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/pipeline.json")
	if err != nil {
		return nil, err
	}
	ans.PipelineAdmin, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/pipeline-admin.json")
	if err != nil {
		return nil, err
	}
	ans.Faucet, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/faucet.json")
	if err != nil {
		return nil, err
	}
	ans.Cranker, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/cranker.json")
	if err != nil {
		return nil, err
	}
	bidderCount := 2
	ans.Bidder = make([]sgo.PrivateKey, bidderCount)
	ans.BidderConfigFilePath = make([]string, bidderCount)
	for i := 0; i < bidderCount; i++ {
		ans.Bidder[i], err = sgo.PrivateKeyFromSolanaKeygenFile(
			fmt.Sprintf("%s/bidder-%d.json", fp, i),
		)
		if err != nil {
			return nil, err
		}
		ans.BidderConfigFilePath[i] = fmt.Sprintf("%s/bidder-%d-config.json", fp, i)
	}

	ans.Cranker, err = sgo.PrivateKeyFromSolanaKeygenFile(fp + "/cranker.json")
	if err != nil {
		return nil, err
	}
	programIdKp, err := sgo.PrivateKeyFromSolanaKeygenFile(fp + "/program.json")
	if err != nil {
		return nil, err
	}

	cba.SetProgramID(programIdKp.PublicKey())
	ans.rpcUrl = "http://127.0.0.1:8899"
	ans.wsUrl = "ws://127.0.0.1:8900"
	ans.Version = vrs.VERSION_1

	ans.PipelineClearNet = &relay.ClearNetListenConfig{
		Port: 50051,
		Ipv4: net.IPv4(127, 0, 0, 1),
		Ipv6: nil,
	}
	ans.ValidatorClearNet = &relay.ClearNetListenConfig{
		Port: 50052,
		Ipv4: net.IPv4(127, 0, 0, 1),
		Ipv6: nil,
	}
	return ans, nil
}

func loopDelete(ctx context.Context, fp string) {
	<-ctx.Done()
	os.Remove(fp)
}
