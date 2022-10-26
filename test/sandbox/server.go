package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	pbt "github.com/solpipe/solpipe-tool/proto/test"
	"github.com/solpipe/solpipe-tool/script"
	vrs "github.com/solpipe/solpipe-tool/state/version"
	sgo "github.com/SolmateDev/solana-go"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const DEFAULT_SOCKET_FILE_PATH = "/tmp/test.socket"

// automatically delete the socket once the context is "done"
func DefaultLocalGrpcListenUrl(ctx context.Context) (string, error) {
	socketFp := DEFAULT_SOCKET_FILE_PATH
	go loopGrpcDelete(ctx, socketFp)
	return socketFp, nil
}

func loopGrpcDelete(ctx context.Context, fp string) {
	<-ctx.Done()
	os.Remove(fp)
}

// set listenUrl="" to get the default unix domain socket available at DefaultLocalGrpcListenUrl
func (ts *TestSetup) CreateGrpc(ctx context.Context, listenUrl string) (signalC <-chan error, err error) {
	var l net.Listener

	ctxInside, cancel := context.WithCancel(ctx)

	if len(listenUrl) == 0 {
		var sockFp string
		sockFp, err = DefaultLocalGrpcListenUrl(ctxInside)
		if err != nil {
			cancel()
			return
		}
		l, err = net.Listen("unix", sockFp)
	} else {
		l, err = net.Listen("tcp", listenUrl)
	}
	if err != nil {
		cancel()
		return
	}
	var s *grpc.Server
	s = grpc.NewServer()
	internalC := make(chan func(*internal), 10)
	listenErrorC := make(chan error, 1)
	badHealthC := make(chan struct{}, 1)
	e1 := external{
		ctx:        ctxInside,
		internalC:  internalC,
		badHealthC: badHealthC,
		wg:         ts.wg,
		version:    ts.Sandbox.version,
		url:        ts.Sandbox.url,
		faucet:     *ts.Sandbox.Faucet,
	}
	pbt.RegisterSandboxServer(s, e1)
	go loopInternal(ctxInside, cancel, internalC, listenErrorC, badHealthC, ts)
	go loopGrpcListen(l, s, listenErrorC)
	go loopGrpcShutdown(ctxInside, s, l)
	signalC = e1.CloseSignal()

	return
}

func loopGrpcShutdown(ctx context.Context, s *grpc.Server, l net.Listener) {
	<-ctx.Done()
	s.GracefulStop()
	l.Close()
}

func loopGrpcListen(l net.Listener, s *grpc.Server, errorC chan<- error) {
	select {
	case errorC <- s.Serve(l):
	}
}

func loopInternal(ctx context.Context, cancel context.CancelFunc, internalC <-chan func(*internal), listenErrorC <-chan error, badHealthC <-chan struct{}, ts *TestSetup) {
	defer cancel()
	var err error
	doneC := ctx.Done()
	errorC := make(chan error, 1)

	in := new(internal)
	in.ctx = ctx
	in.cancel = cancel
	in.errorC = errorC
	in.ts = ts
	in.staker_i = 0
	in.validatorsUsed = make(map[string]*Validator)
	in.count = 0
	in.hasHealthBeenSet = false
out:
	for !in.hasHealthBeenSet || (in.hasHealthBeenSet && 0 < in.count) {
		select {
		case <-doneC:
			break out
		case err = <-listenErrorC:
			break out
		case err = <-errorC:
			break out
		case <-badHealthC:
			err = errors.New("health marked bad")
			break out
		case req := <-internalC:
			req(in)
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

type external struct {
	pbt.UnimplementedSandboxServer
	ctx        context.Context
	internalC  chan<- func(*internal)
	badHealthC chan<- struct{}
	version    vrs.CbaVersion
	url        string
	wg         *sync.WaitGroup
	faucet     sgo.PrivateKey
}

func (e1 external) GetConfig(ctx context.Context, req *pbt.Empty) (resp *pbt.SandboxConfig, err error) {

	resultC := make(chan configResult, 1)
	e1.internalC <- func(in *internal) {
		resultC <- in.get_config()
	}
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case result := <-resultC:
		err = result.err
		resp = result.c
	}
	return
}

type configResult struct {
	c   *pbt.SandboxConfig
	err error
}

func (in *internal) get_config() configResult {
	ans := &pbt.SandboxConfig{}
	var err error
	switch in.ts.Sandbox.url {
	case "localhost":
		ans.Version = string(in.ts.Sandbox.version)
		ans.RpcUrl = "http://localhost:8899"
		ans.WsUrl = "ws://localhost:8900"
		ans.AdminUrl = ""
		ans.Headers = make(map[string]string)
	default:
		err = fmt.Errorf("only localhost works, not %s", in.ts.Sandbox.url)
	}
	return configResult{err: err, c: ans}
}

type internal struct {
	ctx              context.Context
	cancel           context.CancelFunc
	closeSignalCList []chan<- error
	errorC           chan<- error
	ts               *TestSetup
	staker_i         int
	validatorsUsed   map[string]*Validator
	count            int
	hasHealthBeenSet bool
}

func (e1 external) PopStaker(ctx context.Context, req *pbt.Empty) (resp *pbt.Staker, err error) {
	resultC := make(chan stakerResult, 1)
	e1.internalC <- func(in *internal) {
		resultC <- in.pop_staker()
	}
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case x := <-resultC:
		err = x.err
		resp = x.r
	}
	return
}

type stakerResult struct {
	r   *pbt.Staker
	err error
}

func (in *internal) pop_staker() stakerResult {
	if len(in.ts.Sandbox.StakerDb.Stakers) <= in.staker_i {
		return stakerResult{err: errors.New("no more stakers")}
	}
	staker := in.ts.Sandbox.StakerDb.Stakers[in.staker_i]
	in.staker_i++
	admin := make([]byte, len([]byte(staker.Admin)))
	copy(admin, []byte(staker.Admin))
	stake := make([]byte, len([]byte(staker.Stake)))
	copy(stake, []byte(staker.Stake))
	return stakerResult{err: nil, r: &pbt.Staker{
		Stake: stake, Admin: admin,
	}}
}

type validatorResult struct {
	r   *pbt.Validator
	err error
}

func (e1 external) PopValidator(ctx context.Context, req *pbt.Empty) (resp *pbt.Validator, err error) {
	resultC := make(chan validatorResult, 1)
	e1.internalC <- func(in *internal) {
		resultC <- in.pop_validator()
	}
	select {
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	case result := <-resultC:
		err = result.err
		resp = result.r
	}
	return
}

func (in *internal) pop_validator() validatorResult {
	var validator *Validator
	var present bool
out:
	for k, v := range in.ts.Sandbox.ValidatorDb.Validators {
		_, present = in.validatorsUsed[k]
		if !present {
			in.validatorsUsed[k] = v
			validator = v
			break out
		}
	}

	if validator == nil {
		return validatorResult{err: errors.New("no validators")}
	} else {
		id := make([]byte, len(validator.Id))
		copy(id, validator.Id)
		vote := make([]byte, len(validator.Vote))
		copy(vote, validator.Vote)
		admin := make([]byte, len(validator.Admin))
		copy(admin, validator.Admin)
		return validatorResult{err: nil, r: &pbt.Validator{
			Id: id, Vote: vote, Admin: admin,
		}}
	}
}

func (e1 external) mark_bad_health() error {
	err := e1.ctx.Err()
	if err != nil {
		return err
	}
	select {
	case e1.badHealthC <- struct{}{}:
		err = nil
	case <-e1.ctx.Done():
		err = errors.New("canceled")
	}
	return err
}

func (e1 external) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	err := e1.ctx.Err()
	if err != nil {
		signalC <- err
		return signalC
	}
	e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}

func (e1 external) heart_increment() {

	e1.internalC <- func(in *internal) {
		in.hasHealthBeenSet = true
		in.count++
	}
}

func (e1 external) heart_decrement() {
	e1.internalC <- func(in *internal) {
		in.count--
		if in.hasHealthBeenSet && in.count == 0 {
			log.Debugf("marking test server finished")
			in.cancel()
		}
	}
}

func (e1 external) HeartBeat(stream pbt.Sandbox_HeartBeatServer) error {
	doneC := e1.ctx.Done()
	var err error
	e1.wg.Add(1)
	errorC := make(chan error, 1)
	finishC := e1.CloseSignal()
	go e1.heart_increment()
	go loopHeartBeat(stream, errorC)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-finishC:
	case err = <-errorC:
		e1.heart_decrement()
	}

	e1.wg.Done()
	return err
}

func loopHeartBeat(stream pbt.Sandbox_HeartBeatServer, errorC chan<- error) {
	var err error
	for {
		_, err = stream.Recv()
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			break
		}
	}
	errorC <- err
}

func (e1 external) Airdrop(ctx context.Context, req *pbt.AirdropRequest) (resp *pbt.Empty, err error) {
	resp = new(pbt.Empty)

	var dst sgo.PublicKey
	dst, err = sgo.PublicKeyFromBase58(req.Pubkey)
	if err != nil {
		return
	}
	if req.Amount == 0 {
		err = errors.New("amount is 0")
		return
	}
	amount := req.Amount
	errorC := make(chan error, 1)
	scriptC := make(chan *script.Script, 1)
	e1.internalC <- func(in *internal) {
		s1, err2 := script.Create(ctx, &script.Configuration{Version: e1.version}, in.ts.Rpc, in.ts.Ws)
		if err2 != nil {
			errorC <- err2
			return
		}
		faucet := *in.ts.Sandbox.Faucet
		err2 = s1.SetTx(faucet)
		if err2 != nil {
			errorC <- err2
			return
		}
		err2 = s1.Transfer(faucet, dst, amount)
		if err2 != nil {
			errorC <- err2
			return
		}
		errorC <- nil
		scriptC <- s1
	}

	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return
	}
	s := <-scriptC
	err = s.FinishTx(false)
	if err != nil {
		return
	}

	return
}

func (e1 external) CreateStake(ctx context.Context, req *pbt.CreateStakeRequest) (resp *pbt.Empty, err error) {
	resp = new(pbt.Empty)

	funds, err := sgo.PrivateKeyFromBase58(req.Funds)
	if err != nil {
		return
	}
	stake, err := sgo.PrivateKeyFromBase58(req.Stake)
	if err != nil {
		return
	}

	admin, err := sgo.PrivateKeyFromBase58(req.Admin)
	if err != nil {
		return
	}

	rpcC := make(chan *sgorpc.Client, 1)
	wsC := make(chan *sgows.Client, 1)
	e1.internalC <- func(in *internal) {
		wsC <- in.ts.Ws
		rpcC <- in.ts.Rpc
	}
	var rpcClient *sgorpc.Client
	var wsClient *sgows.Client
	select {
	case <-ctx.Done():
		err = errors.New("canceled")
	case wsClient = <-wsC:
		rpcClient = <-rpcC
	}

	if err != nil {
		return
	}
	err = CreateStake(ctx, e1.url, rpcClient, wsClient, funds, stake, admin, req.Amount)
	if err != nil {
		return
	}

	return
}

func (e1 external) DelegateStake(ctx context.Context, req *pbt.DelegateStakeRequest) (resp *pbt.Empty, err error) {
	resp = new(pbt.Empty)

	payer, err := sgo.PrivateKeyFromBase58(req.Payer)
	if err != nil {
		return
	}
	stake, err := sgo.PublicKeyFromBase58(req.Stake)
	if err != nil {
		return
	}
	stakeAdmin, err := sgo.PrivateKeyFromBase58(req.Admin)
	if err != nil {
		return
	}
	vote, err := sgo.PublicKeyFromBase58(req.Vote)
	if err != nil {
		return
	}
	err = DelegateStake(ctx, e1.url, payer, stake, stakeAdmin, vote)
	if err != nil {
		return
	}
	return
}
