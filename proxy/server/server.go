package server

import (
	"context"
	"errors"

	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	rtr "github.com/solpipe/solpipe-tool/state/router"
	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
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
	s *grpc.Server,
	router rtr.Router,
	admin sgo.PrivateKey,
	relay relay.Relay,
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
	pbj.RegisterTransactionServer(s, e1)
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
