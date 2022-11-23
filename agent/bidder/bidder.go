package bidder

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/signal"
	"syscall"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	rtr "github.com/solpipe/solpipe-tool/state/router"
)

type Agent struct {
	ctx        context.Context
	controller ctr.Controller
	bidder     sgo.PrivateKey
	pcVault    sgo.PublicKey
	internalC  chan<- func(*internal)
}

type Configuration struct {
	// what is the maximum that can be bid
	Budget uint64 `json:"max_bid"`
	// what is the target ration between our deposit and the total deposit
	TargetTPS float64 `json:"target_tps"`
	// what is the maximum amount the bidder can change the bid to the positive or negative side (range is 1-255 out of 256)
	MaxDelta float64 `json:"max_delta"`
	GrpcUrl  string  `json:"grpc_url"`
}

func ConfigFromFile(filePath string) (*Configuration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	c := new(Configuration)
	err = json.Unmarshal(data, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Create(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	configChannelGroup ConfigByHubChannelGroup,
	bidder sgo.PrivateKey,
	pcVaultId sgo.PublicKey,
	pcVault *sgotkn.Account,
	router rtr.Router,
) (Agent, error) {
	internalC := make(chan func(*internal), 10)
	startErrorC := make(chan error, 1)

	ctxC, cancel := context.WithCancel(ctx)

	go loopInternal(
		ctxC,
		cancel,
		internalC,
		startErrorC,
		configChannelGroup.ErrorC,
		configChannelGroup.ConfigC,
		rpcClient,
		wsClient,
		bidder,
		pcVaultId,
		pcVault,
		router,
	)

	return Agent{
		ctx:        ctxC,
		internalC:  internalC,
		controller: router.Controller,
	}, nil
}

func (e1 Agent) CloseSignal() <-chan error {
	doneC := e1.ctx.Done()
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
		return signalC
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
		return signalC
	}
}

type ConfigByHubChannelGroup struct {
	ConfigC <-chan Configuration
	ErrorC  <-chan error
}

func CreateConfigPair() (ConfigByHubChannelGroup, chan<- Configuration) {
	configC := make(chan Configuration, 1)
	errorC := make(chan error, 1)
	return ConfigByHubChannelGroup{ConfigC: configC, ErrorC: errorC}, configC
}

// To update the config in a running proces, send SIGHUP to the PID
func ConfigByHup(ctx context.Context, configFilePath string) ConfigByHubChannelGroup {
	configC := make(chan Configuration, 1)
	errorC := make(chan error, 1)
	go loopModifyConfig(ctx, configC, errorC, configFilePath)
	return ConfigByHubChannelGroup{
		ConfigC: configC,
		ErrorC:  errorC,
	}
}

// if someone hups, then read the updated config from the command line
func loopModifyConfig(
	ctx context.Context,
	configC chan<- Configuration,
	configReadErrorC chan<- error,
	configFilePath string,
) {
	doneC := ctx.Done()
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGHUP)
	var err error

	config, err := ConfigFromFile(configFilePath)
	if err != nil {
		configC <- *config
		return
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case <-sigC:
			c, err := ConfigFromFile(configFilePath)
			if err != nil {
				break out
			}
			configC <- *c
		}
	}
	if err != nil {
		configReadErrorC <- err
	}
}
