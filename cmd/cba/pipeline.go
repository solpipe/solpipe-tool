package main

import (
	"errors"
	"fmt"
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
	Agent  PipelineAgent  `cmd name:"agent" help:"run a Pipeline Agent"`
	Status PipelineStatus `cmd name:"status" help:"Print the admin, token balance of the controller"`
}

type PipelineCreate struct {
}

func (r *PipelineCreate) Run(kongCtx *CLIContext) error {
	return errors.New("not implemented yet")
}

type PipelineAgent struct {
	CrankRate        string        `option name:"crank_rate"  help:"the crank rate in the form NUMERATOR/DENOMINATOR"`
	DecayRate        string        `option name:"decay_rate"  help:"the decay rate in the form NUMERATOR/DENOMINATOR"`
	PayoutShare      string        `option name:"payout_share" help:"the payout share in the form NUMERATORDENOMINATOR"`
	AdminUrl         string        `option name:"admin_url" help:"port on which to listen for Grpc connections from administrators."`
	BalanceThreshold uint64        `option name:"balance"  help:"set the minimum balance threshold"`
	ProgramIdCba     sgo.PublicKey `name:"program_id_cba" help:"Specify the program id for the CBA program"`
	PipelineId       string        `arg name:"id" help:"the Pipeline ID"`
	Admin            string        `arg name:"admin" help:"the Pipeline admin"`
}

func convertRate(in string, defaultRate *state.Rate) (*state.Rate, error) {
	var pair [2]uint64
	var err error

	if 0 < len(in) {
		split := strings.Split(in, "/")
		if len(split) != 2 {
			return nil, errors.New("format not N/D")
		}

		for i := 0; i < 2; i++ {
			pair[i], err = strconv.ParseUint(split[i], 10, 8)
			if err != nil {
				return nil, err
			}
		}
	}

	if pair[1] == 0 {
		return defaultRate, nil
	} else {
		return &state.Rate{
			N: pair[0],
			D: pair[1],
		}, nil
	}

}

func (r *PipelineAgent) Run(kongCtx *CLIContext) error {

	ctx := kongCtx.Ctx
	admin, err := sgo.PrivateKeyFromSolanaKeygenFile(r.Admin)
	if err != nil {
		return err
	}

	pipelineConfig := relay.CreateConfiguration(
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
	router, err := pipelineConfig.Router(ctx)
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

	agent, err := ap.Create(
		ctx,
		&ap.InitializationArg{
			Relay: &pipelineConfig,
			Program: &ap.Configuration{
				ProgramIdCba: cba.ProgramID.ToPointer(),
				Pipeline:     pipeline.Id.ToPointer(),
				Wallet:       &pipelineConfig.Admin,
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
		list, err := pipeline.PeriodRingWithPayout()
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
