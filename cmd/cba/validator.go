package main

import (
	"errors"
	"math"
	"net"
	"os"
	"strconv"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	valagent "github.com/solpipe/solpipe-tool/agent/validator"
	valadmin "github.com/solpipe/solpipe-tool/agent/validator/admin"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/proxy/relay"
)

type Validator struct {
	Create   ValidatorCreate   `cmd name:"create" help:"register a validator"`
	Agent    ValidatorAgent    `cmd name:"agent" help:"run a Validator Agent"`
	Pipeline ValidatorPipeline `cmd name:"pipeline" help:"Select a pipeline"`
	Status   ValidatorStatus   `cmd name:"status" help:"Print the admin, token balance of the validator"`
}

type ValidatorCreate struct {
	Payer    string `name:"payer" help:"The private key that owns the SOL to pay the transaction fees."`
	VoteKey  string `arg name:"vote" help:"The vote account for the validator."`
	AdminKey string `arg name:"admin" help:"The admin key used to administrate the validator pipeline."`
	StakeKey string `arg name:"stake" help:"Pick one stake account as a sample."`
}

func (r *ValidatorCreate) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}

	payer, err := sgo.PrivateKeyFromBase58(r.Payer)
	if err != nil {
		payer, err = sgo.PrivateKeyFromSolanaKeygenFile(r.Payer)
		if err != nil {
			return err
		}
	}
	log.Debugf("payer=%s", payer.PublicKey().String())

	vote, err := sgo.PrivateKeyFromBase58(r.VoteKey)
	if err != nil {
		vote, err = sgo.PrivateKeyFromSolanaKeygenFile(r.VoteKey)
		if err != nil {
			return err
		}
	}
	log.Debugf("vote=%s", vote.PublicKey().String())

	stake, err := sgo.PublicKeyFromBase58(r.StakeKey)
	if err != nil {
		return err
	}
	log.Debugf("stake=%s", stake.String())

	admin, err := sgo.PrivateKeyFromBase58(r.AdminKey)
	if err != nil {
		admin, err = sgo.PrivateKeyFromSolanaKeygenFile(r.AdminKey)
		if err != nil {
			return err
		}
	}
	log.Debugf("admin=%s", admin.PublicKey().String())
	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		admin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		DEFAULT_VALIDATOR_ADMIN_SOCKET, // this line is irrelevant
		nil,
	)

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}
	s1, err := relayConfig.ScriptBuilder(ctx)
	if err != nil {
		return err
	}
	err = s1.SetTx(payer)
	if err != nil {
		return err
	}

	_, err = s1.AddValidator(router.Controller.Id(), vote, stake, admin)
	if err != nil {
		return err
	}
	return s1.FinishTx(true)
}

type ValidatorAgent struct {
	ClearListenUrl string `option name:"clear_listen"  help:"url to which clients can connect without tor"`
	AdminListenUrl string `option name:"admin_url" help:"The url on which the admin grpc server listens."`
	VoteKey        string `arg name:"vote" help:"The vote account for the validator."`
	AdminKey       string `arg name:"admin" help:"The admin key used to administrate the validator."`
	ConfigFilePath string `arg name:"configuration" help:"The file path to the configuration."`
}

const DEFAULT_VALIDATOR_ADMIN_SOCKET = "unix:///tmp/.validator.socket"

func (r *ValidatorAgent) Run(kongCtx *CLIContext) error {
	//Create(ctx context.Context, config *Configuration, rpcClient *sgorpc.Client, wsClient *sgows.Client)
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}

	vote, err := sgo.PublicKeyFromBase58(r.VoteKey)
	if err != nil {
		return err
	}

	admin, err := sgo.PrivateKeyFromBase58(r.AdminKey)
	if err != nil {
		admin, err = sgo.PrivateKeyFromSolanaKeygenFile(r.AdminKey)
		if err != nil {
			return err
		}
	}

	adminUrl := DEFAULT_VALIDATOR_ADMIN_SOCKET
	if 0 < len(r.AdminListenUrl) {
		adminUrl = r.AdminListenUrl
	}

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		admin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		adminUrl,
		nil,
	)

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}

	validator, err := router.ValidatorByVote(vote)
	if err != nil {
		return err
	}

	if 0 < len(r.ClearListenUrl) {
		clearConfig := new(relay.ClearNetListenConfig)
		x := strings.Split(r.ClearListenUrl, ":")
		if len(x) != 2 {
			return errors.New("use form HOST:PORT for clear net listen url")
		}
		clearConfig.Ipv4 = net.ParseIP(x[0])
		if clearConfig.Ipv4 == nil {
			return errors.New("failed to parse ip address")
		}
		z, err := strconv.Atoi(x[1])
		if err != nil {
			return err
		}
		if z < 0 || math.MaxUint16 <= z {
			return errors.New("port out of range")
		}
		clearConfig.Port = uint16(z)
		relayConfig.ClearNet = clearConfig
	}
	torMgr, err := proxy.SetupTor(ctx, false)
	if err != nil {
		return err
	}
	go loopCloseTor(ctx, torMgr)
	agent, err := valagent.Create(
		ctx,
		relayConfig,
		router,
		validator,
		r.ConfigFilePath,
		torMgr,
	)
	if err != nil {
		return err
	}
	return <-agent.CloseSignal()
}

type ValidatorStatus struct {
	VoteKey string `name:"vote" help:"The vote account for the validator."`
}

func (r *ValidatorStatus) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}

	vote, err := sgo.PublicKeyFromBase58(r.VoteKey)
	if err != nil {
		return err
	}

	placeHolderDummyKey, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		placeHolderDummyKey,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		"ignored",
		nil,
	)

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}

	validator, err := router.ValidatorByVote(vote)
	if err != nil {
		return err
	}

	content, err := validator.Print()
	if err != nil {
		return err
	}
	os.Stdout.Write([]byte(content))
	return nil
}

type ValidatorPipeline struct {
	AdminListenUrl string `option name:"admin_url" help:"The url on which the admin grpc server listens."`
	PipelineKey    string `arg name:"pipeline" help:"The pipeline to which to assign the validator bandwidth."`
}

func (r *ValidatorPipeline) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	if kongCtx.Clients == nil {
		return errors.New("no rpc or ws client")
	}

	pipelineId, err := sgo.PublicKeyFromBase58(r.PipelineKey)
	if err != nil {
		return err
	}

	adminUrl := DEFAULT_VALIDATOR_ADMIN_SOCKET
	if 0 < len(r.AdminListenUrl) {
		adminUrl = r.AdminListenUrl
	}

	placeHolderDummyKey, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		placeHolderDummyKey,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		adminUrl,
		nil,
	)

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}

	pipeline, err := router.PipelineById(pipelineId)
	if err != nil {
		return err
	}

	a, err := valadmin.Dial(ctx, adminUrl)
	if err != nil {
		return err
	}

	return a.SetPipeline(ctx, pipeline.Id)
}
