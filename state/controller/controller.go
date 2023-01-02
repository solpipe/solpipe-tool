package controller

import (
	"context"
	"errors"
	"fmt"

	sgo "github.com/SolmateDev/solana-go"
	sgotkn "github.com/SolmateDev/solana-go/programs/token"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	bin "github.com/gagliardetto/binary"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	sub2 "github.com/solpipe/solpipe-tool/ds/sub"
	slt "github.com/solpipe/solpipe-tool/state/slot"
	"github.com/solpipe/solpipe-tool/state/sub"
	vrs "github.com/solpipe/solpipe-tool/state/version"
)

type Controller struct {
	ctx               context.Context
	id                sgo.PublicKey
	internalC         chan<- func(*controllerInternal)
	Version           vrs.CbaVersion
	updateControllerC chan<- sub2.ResponseChannel[cba.Controller]
	rpc               *sgorpc.Client
	ws                *sgows.Client
	subSlot           slt.SlotHome
}

func (e1 Controller) Print() (string, error) {
	ans := ""

	ans += fmt.Sprintf("Account ID=%s\n", e1.id.String())
	data, err := e1.Data()
	if err != nil {
		return "", err
	}
	ans += fmt.Sprintf("Admin=%+v\n", data.Admin.String())
	ans += fmt.Sprintf("Vault Mint=%+v\n", data.PcMint.String())
	ans += fmt.Sprintf("Vault Id=%+v\n", data.PcVault.String())
	a := new(sgotkn.Account)
	x, err := e1.rpc.GetAccountInfo(e1.ctx, data.PcVault)
	if err != nil {
		return "", err
	}
	err = bin.UnmarshalBorsh(a, x.Value.Data.GetBinary())
	if err != nil {
		return "", err
	}
	ans += fmt.Sprintf("Vault Balance=%d\n", a.Amount)

	return ans, nil
}

func (e1 Controller) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	e1.internalC <- func(in *controllerInternal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}
	return signalC
}

func CreateController(ctx context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client, version vrs.CbaVersion) (Controller, error) {
	controllerId, _, err := vrs.ControllerId(version)
	if err != nil {
		return Controller{}, err
	}

	all, err := sub.FetchProgramAll(ctx, rpcClient, version)
	if err != nil {
		return Controller{}, err
	}

	log.Infof("controller id 2 =%s", controllerId.String())
	data := all.Controller

	subSlot, err := slt.SubscribeSlot(ctx, rpcClient, wsClient)
	if err != nil {
		return Controller{}, err
	}

	internalC := make(chan func(*controllerInternal), 10)

	sub, err := wsClient.AccountSubscribe(controllerId, sgorpc.CommitmentConfirmed)
	if err != nil {
		return Controller{}, err
	}
	home := sub2.CreateSubHome[cba.Controller]()
	reqC := home.ReqC
	go loopController(ctx, internalC, controllerId, *data, sub, home)

	return Controller{ctx: ctx, subSlot: subSlot, id: controllerId, internalC: internalC, updateControllerC: reqC, rpc: rpcClient, ws: wsClient, Version: version}, nil
}

func (e1 Controller) Id() sgo.PublicKey {
	return e1.id
}

func (e1 Controller) SlotHome() slt.SlotHome {
	return e1.subSlot
}

func (e1 Controller) filter_onController(c cba.Controller) bool {
	return true
}

func (e1 Controller) OnData() sub2.Subscription[cba.Controller] {
	return sub2.SubscriptionRequest(e1.updateControllerC, e1.filter_onController)
}

func (e1 Controller) Data() (cba.Controller, error) {
	errorC := make(chan error, 1)
	ansC := make(chan cba.Controller, 1)
	e1.internalC <- func(in *controllerInternal) {
		if in.data == nil {
			errorC <- errors.New("no controller information")
		} else {
			errorC <- nil
			ansC <- *in.data
		}
	}
	err := <-errorC
	if err != nil {
		return cba.Controller{}, err
	}
	return <-ansC, nil
}

type controllerInternal struct {
	ctx              context.Context
	closeSignalCList []chan<- error
	errorC           chan<- error
	id               sgo.PublicKey
	data             *cba.Controller
	home             *sub2.SubHome[cba.Controller]
}

func loopController(
	ctx context.Context,
	internalC <-chan func(*controllerInternal),
	id sgo.PublicKey,
	data cba.Controller,
	sub *sgows.AccountSubscription,
	home *sub2.SubHome[cba.Controller],
) {
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	in := new(controllerInternal)
	in.ctx = ctx
	in.errorC = errorC
	in.closeSignalCList = make([]chan<- error, 0)
	in.id = id
	in.data = &data
	in.home = home

	streamC := sub.RecvStream()
	closeC := sub.RecvErr()

	var err error
out:
	for {
		select {
		case x := <-in.home.ReqC:
			in.home.Receive(x)
		case d := <-streamC:
			r, ok := d.(*sgows.AccountResult)
			if !ok {
				err = errors.New("bad account result")
				break out
			}
			data := new(cba.Controller)
			err = bin.UnmarshalBorsh(data, r.Value.Account.Data.GetBinary())
			if err != nil {
				break out
			}
			in.on_data(data)
		case err = <-closeC:
			break out
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (in *controllerInternal) on_data(data *cba.Controller) {
	in.data = data
}
