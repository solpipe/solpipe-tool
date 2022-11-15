package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/alecthomas/kong"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type CLIContext struct {
	Clients *Clients
	Ctx     context.Context
}

type debugFlag bool

type ProgramIdCba string

type ApiKey string
type Version string
type RpcUrl string
type WsUrl string

var cli struct {
	Verbose      debugFlag    `help:"Set logging to verbose." short:"v" default:"false"`
	ProgramIdCba ProgramIdCba `option name:"program-id" default:"2nV2HN9eaaoyk4WmiiEtUShup9hVQ21rNawfor9qoqam" help:"Program ID for the CBA Solana program"`
	Version      Version      `option name:"version" help:"What version is the controller"`
	RpcUrl       RpcUrl       `option name:"rpc" help:"Connection information to a Solana validator Rpc endpoint with format protocol://host:port (ie http://localhost:8899)"`
	WsUrl        WsUrl        `option name:"ws" help:"Connection information to a Solana validator Websocket endpoint with format protocol://host:port (ie ws://localhost:8900)" type:"string"`
	ApiKey       ApiKey       `option name:"apikey" help:"An API Key used to connect to an RPC Provider"`
	Cranker      Cranker      `cmd name:"cranker" help:"Crank the CBA program"`
	Bidder       Bidder       `cmd name:"bid" help:"Bid for transaction bandwidth."`
	Controller   Controller   `cmd name:"controller" help:"Manage the controller"`
	Pipeline     Pipeline     `cmd name:"pipeline" help:"Run a JSON RPC send_tx proxy"`
	Validator    Validator    `cmd name:"validator" help:"Run a JSON RPC send_tx proxy"`
}

// PROGRAM_ID_CBA=2nV2HN9eaaoyk4WmiiEtUShup9hVQ21rNawfor9qoqam

type Clients struct {
	ctx     context.Context
	RpcUrl  string
	WsUrl   string
	Headers http.Header
	Version vrs.CbaVersion
}

func (v RpcUrl) AfterApply(clients *Clients) error {
	clients.RpcUrl = string(v)
	return nil
}

func (v WsUrl) AfterApply(clients *Clients) error {
	clients.WsUrl = string(v)
	return nil
}

func (v Version) AfterApply(clients *Clients) error {
	switch v {
	case "1":
		clients.Version = vrs.VERSION_1
	default:
		return errors.New("bad version")
	}
	return nil
}

func (key ApiKey) AfterApply(clients *Clients) error {
	if len(key) == 0 {
		clients.Headers = http.Header{}
	} else {
		return errors.New("not implemented yet")
	}
	return nil
}

func (idstr ProgramIdCba) AfterApply(clients *Clients) error {
	id, err := sgo.PublicKeyFromBase58(string(idstr))
	if err != nil {
		return err
	}
	cba.SetProgramID(id)
	return nil
}

func (d debugFlag) AfterApply(clients *Clients) error {
	if d {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	return nil
}

func main() {

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGTERM, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	go loopSignal(ctx, cancel, signalC)
	clients := &Clients{ctx: ctx}
	kongCtx := kong.Parse(&cli, kong.Bind(clients))
	if len(clients.Version) == 0 {
		clients.Version = vrs.VERSION_1
	}
	err := kongCtx.Run(&CLIContext{Ctx: ctx, Clients: clients})
	kongCtx.FatalIfErrorf(err)
}

func loopSignal(ctx context.Context, cancel context.CancelFunc, signalC <-chan os.Signal) {
	defer cancel()
	doneC := ctx.Done()
	select {
	case <-doneC:
	case s := <-signalC:
		os.Stderr.WriteString(fmt.Sprintf("%s\n", s.String()))
	}
}

func readRate(rate string) (*state.Rate, error) {
	ans := new(state.Rate)
	var err error
	x := strings.Split(rate, "/")
	if len(x) != 2 {
		return nil, errors.New("fee is not of form numerator/denominator")
	}
	ans.N, err = strconv.ParseUint(x[0], 10, 64)
	if err != nil {
		return nil, err
	}
	ans.D, err = strconv.ParseUint(x[1], 10, 64)
	if err != nil {
		return nil, err
	}
	return ans, nil
}

// argument is either file name or private key in base58
func readPrivateKey(input string) (ans sgo.PrivateKey, err error) {

	ans, err = sgo.PrivateKeyFromBase58(input)
	if err != nil {
		ans, err = sgo.PrivateKeyFromSolanaKeygenFile(input)
		if err != nil {
			return
		}
	}
	return
}
