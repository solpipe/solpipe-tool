package util

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strconv"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

func GetGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

type RpcConfig struct {
	Rpc     string
	Ws      string
	Headers http.Header
}

func RpcConfigFromEnv() (*RpcConfig, error) {
	var present bool
	config := new(RpcConfig)
	config.Rpc, present = os.LookupEnv("RPC_URL")
	if !present {
		return nil, errors.New("no rpc url")
	}
	config.Ws, present = os.LookupEnv("WS_URL")
	if !present {
		return nil, errors.New("no ws url")
	}
	return config, nil
}

func RpcConnect(ctx context.Context, config *RpcConfig) (*sgorpc.Client, *sgows.Client, error) {
	var err error
	if config == nil {
		config, err = RpcConfigFromEnv()
		if err != nil {
			return nil, nil, err
		}
	}
	if config.Headers == nil {
		config.Headers = http.Header{}
	}
	rpcClient := sgorpc.NewWithHeaders(config.Rpc, config.Headers)
	wsClient, err := sgows.ConnectWithHeaders(ctx, config.Ws, config.Headers)
	if err != nil {
		return nil, nil, err
	}
	return rpcClient, wsClient, nil
}
