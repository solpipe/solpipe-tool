package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ap "github.com/solpipe/solpipe-tool/agent/pipeline"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"github.com/solpipe/solpipe-tool/state"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
)

type Pipeline struct {
	Create PipelineCreate `cmd name:"create" help:"create a pipeline"`
	Update PipelineUpdate `cmd name:"update" help:"change the settings on a pipeline"`
	Agent  PipelineAgent  `cmd name:"agent" help:"run a Pipeline Agent"`
	Status PipelineStatus `cmd name:"status" help:"Print the admin, token balance of the controller"`
}

type PipelineCreate struct {
	Payer       string `name:"payer" short:"p" help:"the account paying SOL fees"`
	PipelineKey string `arg name:"pipeline" help:"the Pipeline ID private key"`
	AdminKey    string `arg name:"admin" short:"a" help:"the account with administrative privileges"`
	CrankFee    string `arg name:"crank" help:"set the fee that the controller earns from Validator revenue."`
	DecayRate   string `arg name:"decay"  help:"set the fee that the controller earns from Validator revenue."`
	PayoutShare string `arg name:"payout"  help:"set the fee that the controller earns from Validator revenue."`
	Allotment   uint16 `arg name:"allotment" help:"allotment"`
}

func (r *PipelineCreate) Run(kongCtx *CLIContext) error {
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

	admin, err := readPrivateKey(r.AdminKey)
	if err != nil {
		return err
	}

	var pipeline sgo.PrivateKey
	pipeline, err = sgo.PrivateKeyFromBase58(r.PipelineKey)
	if err != nil {
		pipeline, err = sgo.PrivateKeyFromSolanaKeygenFile(r.PipelineKey)
		if err != nil {
			return err
		}
	}

	crankerFee, err := readRate(r.CrankFee)
	if err != nil {
		return err
	}

	decayRate, err := readRate(r.DecayRate)
	if err != nil {
		return err
	}

	payoutShare, err := readRate(r.PayoutShare)
	if err != nil {
		return err
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

	_, err = s1.AddPipelineDirect(
		pipeline,
		router.Controller,
		payer,
		admin,
		*crankerFee,
		r.Allotment,
		*decayRate,
		*payoutShare,
	)
	if err != nil {
		return err
	}

	err = s1.FinishTx(true)
	if err != nil {
		return err
	}
	ans := new(PipelineCreateResponse)
	ans.Pipeline = pipeline.PublicKey().String()
	err = json.NewEncoder(os.Stdout).Encode(ans)
	if err != nil {
		return err
	}
	return nil
}

type PipelineCreateResponse struct {
	Pipeline string `json:"pipeline"`
}

type PipelineUpdate struct {
	Payer       string `name:"payer" short:"p" help:"the account paying SOL fees"`
	PipelineId  string `arg name:"pipeline" help:"the Pipeline ID public key"`
	AdminKey    string `arg name:"admin" short:"a" help:"the account with administrative privileges"`
	CrankFee    string `arg name:"crank" help:"set the fee that the controller earns from Validator revenue."`
	DecayRate   string `arg name:"decay"  help:"set the fee that the controller earns from Validator revenue."`
	PayoutShare string `arg name:"payout"  help:"set the fee that the controller earns from Validator revenue."`
	Allotment   uint16 `arg name:"allotment" help:"allotment"`
}

func (r *PipelineUpdate) Run(kongCtx *CLIContext) error {
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

	admin, err := sgo.PrivateKeyFromBase58(r.AdminKey)
	if err != nil {
		admin, err = sgo.PrivateKeyFromSolanaKeygenFile(r.AdminKey)
		if err != nil {
			return err
		}
	}

	pipeline, err := sgo.PublicKeyFromBase58(r.PipelineId)
	if err != nil {
		return err
	}

	crankerFee, err := readRate(r.CrankFee)
	if err != nil {
		return err
	}

	decayRate, err := readRate(r.DecayRate)
	if err != nil {
		return err
	}

	payoutShare, err := readRate(r.PayoutShare)
	if err != nil {
		return err
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

	err = s1.UpdatePipeline(
		router.Controller.Id(),
		pipeline,
		admin,
		*crankerFee,
		r.Allotment,
		*decayRate,
		*payoutShare,
	)
	if err != nil {
		return err
	}

	err = s1.FinishTx(true)
	if err != nil {
		return err
	}

	return nil
}

type PipelineAgent struct {
	ClearListenUrl   string        `option name:"clear_listen"  help:"url to which clients can connect without tor"`
	CrankRate        string        `option name:"crank_rate"  help:"the crank rate in the form NUMERATOR/DENOMINATOR"`
	DecayRate        string        `option name:"decay_rate"  help:"the decay rate in the form NUMERATOR/DENOMINATOR"`
	PayoutShare      string        `option name:"payout_share" help:"the payout share in the form NUMERATORDENOMINATOR"`
	AdminUrl         string        `option name:"admin_url" help:"port on which to listen for Grpc connections from administrators."`
	BalanceThreshold uint64        `option name:"balance"  help:"set the minimum balance threshold"`
	ProgramIdCba     sgo.PublicKey `name:"program_id_cba" help:"Specify the program id for the CBA program"`
	PipelineId       string        `arg name:"id" help:"the Pipeline ID"`
	Admin            string        `arg name:"admin" help:"the Pipeline admin"`
}

func (r *PipelineAgent) Run(kongCtx *CLIContext) error {

	ctx := kongCtx.Ctx
	admin, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Admin)
	if err != nil {
		return err
	}

	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		admin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		r.AdminUrl,
		nil,
	)
	pipelineId, err := sgo.PublicKeyFromBase58(r.PipelineId)
	if err != nil {
		return err
	}
	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}
	pipeline, err := router.PipelineById(pipelineId)
	if err != nil {
		return err
	}

	crankFee, err := convertRate(r.CrankRate, &state.Rate{N: 1, D: 100})
	if err != nil {
		return err
	}
	decayRate, err := convertRate(r.CrankRate, &state.Rate{N: 1, D: 100})
	if err != nil {
		return err
	}
	payoutShare, err := convertRate(r.CrankRate, &state.Rate{N: 95, D: 100})
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

	agent, err := ap.Create(
		ctx,
		&ap.InitializationArg{
			Relay: &relayConfig,
			Program: &ap.Configuration{
				ProgramIdCba: cba.ProgramID.ToPointer(),
				Pipeline:     pipeline.Id.ToPointer(),
				Wallet:       &relayConfig.Admin,
				Settings: &pipe.PipelineSettings{
					CrankFee:    crankFee,
					DecayRate:   decayRate,
					PayoutShare: payoutShare,
				},
			},
		},
		router,
		pipeline,
	)
	log.Debug("create - 8")
	if err != nil {
		return err
	}
	log.Debug("create - 9")
	<-agent.CloseSignal()
	log.Debug("create - 10")
	return nil
}

type PipelineStatus struct {
	ProgramIdCba sgo.PublicKey `name:"program_id_cba" help:"Specify the program id for the CBA program"`
	PipelineId   string        `arg name:"id" help:"the Pipeline ID"`
}

func (r *PipelineStatus) Run(kongCtx *CLIContext) error {
	ctx := kongCtx.Ctx
	//version := vrs.VERSION_1
	admin, err := sgo.NewRandomPrivateKey()
	if err != nil {
		return err
	}
	relayConfig := relay.CreateConfiguration(
		kongCtx.Clients.Version,
		admin,
		kongCtx.Clients.RpcUrl,
		kongCtx.Clients.WsUrl,
		kongCtx.Clients.Headers.Clone(),
		"",
		nil,
	)
	pipelineId, err := sgo.PublicKeyFromBase58(r.PipelineId)
	if err != nil {
		return err
	}

	router, err := relayConfig.Router(ctx)
	if err != nil {
		return err
	}
	pipeline, err := router.PipelineById(pipelineId)
	if err != nil {
		return err
	}
	str, err := pipeline.Print()
	if err != nil {
		return err
	}
	os.Stdout.WriteString(str + "\n")
	{
		list, err := pipeline.PayoutWithData()
		if err != nil {
			return err
		}
		os.Stdout.WriteString(fmt.Sprintf("period count=%d\n", len(list)))
		for _, p := range list {
			data := p.Data
			start := data.Period.Start
			length := data.Period.Length
			finish := start + length - 1

			os.Stdout.WriteString(fmt.Sprintf("\tpayout id=%s; start=%d; length=%d; end=%d\n", p.Id.String(), start, length, finish))
		}
	}

	{
		list, err := pipeline.PeriodRing()
		if err != nil {
			return err
		}
		os.Stdout.WriteString(fmt.Sprintf("2 - period count=%d\n", list.Length))
		for _, data := range list.Ring {
			start := data.Period.Start
			length := data.Period.Length
			finish := start + length - 1

			os.Stdout.WriteString(fmt.Sprintf("\tpayout id(%t)=%s; start=%d; length=%d; end=%d\n", data.Period.IsBlank, data.Payout.String(), start, length, finish))
		}
	}

	return nil
}
