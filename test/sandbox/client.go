package sandbox

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	pbt "github.com/solpipe/solpipe-tool/proto/test"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	ctr "github.com/solpipe/solpipe-tool/state/controller"
	ntk "github.com/solpipe/solpipe-tool/state/network"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"google.golang.org/grpc"
)

type TestChild struct {
	Ctx            context.Context
	Cancel         context.CancelFunc
	Client         pbt.SandboxClient
	Version        vrs.CbaVersion
	Rpc            *sgorpc.Client
	Ws             *sgows.Client
	RpcUrl         string
	WsUrl          string
	AdminListenUrl string
	Headers        http.Header
	Network        ntk.Network
	Router         rtr.Router
}

// url reverts to default if url="".
func Dial(ctxParent context.Context, url string) (*TestChild, error) {
	ctx, cancel := context.WithCancel(ctxParent)
	var err error
	var conn *grpc.ClientConn
	if len(url) == 0 {
		url = fmt.Sprintf("unix://%s", DEFAULT_SOCKET_FILE_PATH)
	}
	ctxC, cancelC := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelC()
	conn, err = grpc.DialContext(ctxC, url, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		cancel()
		return nil, err
	}
	time.Sleep(10 * time.Second)
	go loopGrpcCloseClient(ctx, conn)

	e1 := &TestChild{
		Ctx:    ctx,
		Cancel: cancel,
		Client: pbt.NewSandboxClient(conn),
	}
	err = e1.get_rpc()
	if err != nil {
		return nil, err
	}
	var controller ctr.Controller
	controller, err = ctr.CreateController(ctx, e1.Rpc, e1.Ws, e1.Version)
	if err != nil {
		return nil, err
	}
	network, err := ntk.Create(ctx, controller, e1.Rpc, e1.Ws)
	if err != nil {
		return nil, err
	}
	e1.Router, err = rtr.CreateRouter(ctx, network, e1.Rpc, e1.Ws, nil, e1.Version)
	if err != nil {
		return nil, err
	}
	stream, err := e1.Client.HeartBeat(ctx)
	if err != nil {
		return nil, err
	}
	go loopClientHeartBeatStream(ctx, cancel, stream)
	return e1, nil
}

func (e1 *TestChild) Controller() ctr.Controller {
	return e1.Router.Controller
}

func loopClientHeartBeatStream(ctx context.Context, cancel context.CancelFunc, stream pbt.Sandbox_HeartBeatClient) {

	go loopClientHeartBeatStreamRead(cancel, stream)
	defer cancel()
	interval := 30 * time.Second

	nextHeartBeat := time.Now().Add(interval)
	var err error

out:
	for {
		select {
		case <-ctx.Done():
			break out
		case <-time.After(time.Until(nextHeartBeat)):
			nextHeartBeat = time.Now().Add(interval)
			err = stream.Send(&pbt.Empty{})
			if err != nil {
				break out
			}
		}
	}
}

func loopClientHeartBeatStreamRead(cancel context.CancelFunc, stream pbt.Sandbox_HeartBeatClient) {
	defer cancel()
	var err error
out:
	for {
		_, err = stream.Recv()
		if err == io.EOF {
			err = nil
			break out
		} else if err != nil {
			break out
		}
	}

}

func loopGrpcCloseClient(ctx context.Context, conn *grpc.ClientConn) {
	<-ctx.Done()
	conn.Close()
}

func (e1 *TestChild) get_rpc() error {
	config, err := e1.Client.GetConfig(e1.Ctx, &pbt.Empty{})
	if err != nil {
		return err
	}

	h := http.Header{}
	if config.Headers != nil {
		for k, v := range config.Headers {
			h[k] = []string{v}
		}
	}
	e1.Version = vrs.CbaVersion(config.Version)
	e1.RpcUrl = config.RpcUrl
	e1.WsUrl = config.WsUrl
	e1.Headers = h.Clone()
	e1.Rpc = sgorpc.NewWithHeaders(config.RpcUrl, h.Clone())
	e1.Ws, err = sgows.ConnectWithHeaders(e1.Ctx, config.WsUrl, h.Clone())
	if len(config.AdminUrl) == 0 {
		dir, err := ioutil.TempDir(os.TempDir(), "client-*")
		if err != nil {
			return err
		}
		e1.AdminListenUrl = "unix://" + dir + "/sandbox.socket"
	} else {
		e1.AdminListenUrl = config.AdminUrl
	}

	if err != nil {
		return err
	}
	return nil
}

func (e1 *TestChild) RelayConfig(admin sgo.PrivateKey) relay.Configuration {
	return relay.CreateConfiguration(
		e1.Version,
		admin,
		e1.RpcUrl,
		e1.WsUrl,
		e1.Headers.Clone(),
		e1.AdminListenUrl,
	)
}

func (e1 *TestChild) DelegateStake(payer sgo.PrivateKey, admin sgo.PrivateKey, stake sgo.PublicKey, vote sgo.PublicKey) error {

	_, err := e1.Client.DelegateStake(e1.Ctx, &pbt.DelegateStakeRequest{
		Stake: stake.String(),
		Admin: admin.String(),
		Payer: payer.String(),
		Vote:  vote.String(),
	})

	return err
}

func (e1 *TestChild) CreateStake(payer sgo.PrivateKey, admin sgo.PrivateKey, amount uint64) (stake sgo.PrivateKey, err error) {
	stake, err = sgo.NewRandomPrivateKey()
	if err != nil {
		return
	}
	_, err = e1.Client.CreateStake(e1.Ctx, &pbt.CreateStakeRequest{
		Funds:  payer.String(),
		Stake:  stake.String(),
		Admin:  admin.String(),
		Amount: amount,
	})
	if err != nil {
		return
	}
	return
}

func (e1 *TestChild) Staker() (*Staker, error) {
	s, err := e1.Client.PopStaker(e1.Ctx, &pbt.Empty{})
	if err != nil {
		return nil, err
	}
	staker := new(Staker)
	staker.Admin = sgo.PrivateKey(s.Admin)
	staker.Stake = sgo.PrivateKey(s.Stake)
	return staker, nil
}

// pop a validator off of the queue
func (e1 *TestChild) PopValidator() (*Validator, error) {
	s, err := e1.Client.PopValidator(e1.Ctx, &pbt.Empty{})
	if err != nil {
		return nil, err
	}
	v := new(Validator)
	v.Admin = sgo.PrivateKey(s.Admin)
	v.Vote = sgo.PrivateKey(s.Vote)
	v.Id = sgo.PrivateKey(s.Id)
	return v, nil
}
