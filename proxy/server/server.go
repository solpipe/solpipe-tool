package server

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	"google.golang.org/grpc"
)

type external struct {
	pbj.UnimplementedTransactionServer
	ctx           context.Context
	internalC     chan<- func(*internal)
	router        rtr.Router
	admin         sgo.PrivateKey
	returnUpdateC chan<- UpdateRequest
	relay         relay.Relay
}

// Listen for connections sending transactions and receipt updates.
// Handle the shutdown of the grpc connection separately.
func Attach(
	ctx context.Context,
	sList []*grpc.Server,
	router rtr.Router,
	admin sgo.PrivateKey,
	relay relay.Relay,
	clearNetConfig *relay.ClearNetListenConfig,
) error {
	ctx2, cancel := context.WithCancel(ctx)
	log.Debugf("creating tor server with key=%s", admin.PublicKey().String())
	internalC := make(chan func(*internal), 10)

	go loopInternal(ctx2, cancel, internalC, router)
	e1 := external{
		ctx:       ctx2,
		internalC: internalC,
		router:    router,
		admin:     admin,
		relay:     relay,
	}
	for i := 0; i < len(sList); i++ {
		s := sList[i]
		pbj.RegisterTransactionServer(s, e1)
		if clearNetConfig != nil {
			pbj.RegisterEndpointServer(s, endpointExternal{
				clearNetConfig: *clearNetConfig,
			})
		}
	}

	return nil
}

func (e1 external) send_cb(
	ctx context.Context,
	cb func(in *internal),
) error {
	doneC := ctx.Done()
	err := ctx.Err()
	if err != nil {
		return err
	}
	select {
	case <-doneC:
		return errors.New("canceled")
	case e1.internalC <- cb:
		return nil
	}
}

type endpointExternal struct {
	pbj.UnimplementedEndpointServer
	clearNetConfig relay.ClearNetListenConfig
}

func (e1 endpointExternal) GetClearNetAddress(
	ctx context.Context,
	req *pbj.EndpointRequest,
) (resp *pbj.EndpointResponse, err error) {
	resp = new(pbj.EndpointResponse)
	resp.Address = new(pbj.Address)
	if e1.clearNetConfig.Ipv4 != nil {
		resp.Address.Ipv4 = e1.clearNetConfig.Ipv4.String()
	} else {
		resp.Address.Ipv4 = ""
	}
	if e1.clearNetConfig.Ipv6 != nil {
		resp.Address.Ipv6 = e1.clearNetConfig.Ipv6.String()
	} else {
		resp.Address.Ipv6 = ""
	}
	resp.Address.Port = uint32(e1.clearNetConfig.Port)
	return
}
