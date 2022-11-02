package relay

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"

	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	"github.com/SolmateDev/solana-go/rpc/jsonrpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/solpipe/solpipe-tool/script"
	"github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type ClearNetListenConfig struct {
	Port uint16
	Ipv4 net.IP
	Ipv6 net.IP
}

type Configuration struct {
	Version        vrs.CbaVersion
	Admin          sgo.PrivateKey
	rpcUrl         string
	wsUrl          string
	headers        http.Header
	AdminListenUrl string
	ClearNet       *ClearNetListenConfig
}

// http headers are copied
func CreateConfiguration(
	version vrs.CbaVersion,
	admin sgo.PrivateKey,
	rpcUrl string,
	wsUrl string,
	headers http.Header,
	adminListenUrl string,
	clearNet *ClearNetListenConfig,
) Configuration {
	h := http.Header{}
	for k, v := range headers {
		h[k] = v
	}
	return Configuration{
		Version:        version,
		Admin:          admin,
		rpcUrl:         rpcUrl,
		wsUrl:          wsUrl,
		headers:        h,
		AdminListenUrl: adminListenUrl,
		ClearNet:       clearNet,
	}
}

func (config Configuration) Check() error {
	if config.headers == nil {
		config.headers = http.Header{}
	}
	if len(config.AdminListenUrl) == 0 {
		return errors.New("no admin listen url")
	}
	if len(config.rpcUrl) == 0 {
		return errors.New("no rpc url")
	}
	if len(config.wsUrl) == 0 {
		return errors.New("no websocket url")
	}
	return nil
}

func (config Configuration) Rpc() *sgorpc.Client {
	h := make(map[string]string)
	for k, v := range config.headers {
		if len(v) == 1 {
			h[k] = v[0]
		}
	}
	return sgorpc.NewWithOpts(config.rpcUrl, &jsonrpc.RPCClientOpts{
		CustomHeaders: h,
	})
}

func (config Configuration) Ws(ctx context.Context) (*sgows.Client, error) {

	return sgows.ConnectWithOptions(ctx, config.wsUrl, &sgows.Options{HttpHeader: config.headers.Clone()})
}

func (config Configuration) ScriptBuilder(ctx context.Context) (*script.Script, error) {
	wsClient, err := config.Ws(ctx)
	if err != nil {
		return nil, err
	}
	return script.Create(ctx, &script.Configuration{Version: config.Version}, config.Rpc(), wsClient)
}

func (config Configuration) AdminListener(ctx context.Context) (l net.Listener, err error) {

	if strings.HasPrefix(config.AdminListenUrl, "tcp") {
		l, err = net.Listen("tcp", config.AdminListenUrl[len("tcp://"):])
		if err != nil {
			return
		}
		go loopCloseListener(ctx, l, "")
		return
	} else if strings.HasPrefix(config.AdminListenUrl, "unix") {
		l, err = net.Listen("unix", config.AdminListenUrl[len("unix://"):])
		if err != nil {
			return
		}
		go loopCloseListener(ctx, l, config.AdminListenUrl[len("unix://"):])
		return
	} else {
		return nil, errors.New("url must be in form of tcp://HOST:PORT or unix:///my/file/path")
	}
}

func loopCloseListener(ctx context.Context, l net.Listener, fp string) {
	<-ctx.Done()
	l.Close()
	if 0 < len(fp) {
		os.Remove(fp)
	}
}

func (config Configuration) Router(ctx context.Context) (r rtr.Router, err error) {
	rpcClient := config.Rpc()
	wsClient, err := config.Ws(ctx)
	if err != nil {
		return
	}
	controller, err := controller.CreateController(ctx, rpcClient, wsClient, config.Version)
	if err != nil {
		return
	}
	network, err := ntk.Create(ctx, controller, rpcClient, wsClient)
	if err != nil {
		return
	}
	r, err = rtr.CreateRouter(ctx, network, rpcClient, wsClient, nil, config.Version)
	if err != nil {
		return
	}
	return
}
