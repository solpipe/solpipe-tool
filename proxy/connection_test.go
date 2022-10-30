package proxy_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"google.golang.org/grpc"
)

func TestClearNet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
	})
	pipeline, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	bidder, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(50051)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	errorC := make(chan error, 1)

	{
		var l net.Listener
		innerListener, err := proxy.CreateListenerLocal(
			ctx,
			fmt.Sprintf("0.0.0.0:%d", port),
			[]string{address},
		)
		if err != nil {
			t.Fatal(err)
		}

		l, err = proxy.CreateListener(
			ctx,
			pipeline,
			innerListener,
		)
		if err != nil {
			t.Fatal(err)
		}
		s := grpc.NewServer()
		attach(ctx, s)

		go loopListenClose(ctx, l)
		go loopListen(errorC, l, s)

	}

	go func() {
		var conn *grpc.ClientConn
		conn, err = proxy.CreateConnectionClearNet(
			ctx,
			pipeline.PublicKey(),
			address,
			bidder,
		)
		if err != nil {
			errorC <- err
			return
		}
		client := pbj.NewEndpointClient(conn)
		resp, err := client.GetClearNetAddress(ctx, &pbj.EndpointRequest{
			Certificate: []byte{},
			Pubkey:      []byte{},
			Nonce:       []byte{},
			Signature:   []byte{},
		})
		if err != nil {
			errorC <- err
			return
		}
		log.Debugf("resp=%+v", resp)
	}()

	select {
	case <-ctx.Done():
	case err = <-errorC:
		if err != nil {
			t.Fatal(err)
		}
	}
}

func loopListenClose(ctx context.Context, l net.Listener) {
	<-ctx.Done()
	l.Close()
}

func loopListen(errorC chan<- error, l net.Listener, s *grpc.Server) {
	err := s.Serve(l)
	if err != nil {
		errorC <- err
	}
}

type external struct {
	pbj.UnimplementedEndpointServer
}

func attach(ctx context.Context, s *grpc.Server) error {
	e1 := external{}
	pbj.RegisterEndpointServer(s, e1)
	return nil
}

func (e1 external) GetClearNetAddress(ctx context.Context, req *pbj.EndpointRequest) (resp *pbj.EndpointResponse, err error) {
	log.Debug("where am I stuck?")
	pubkey, err := relay.GetPeerPubkey(ctx)
	if err != nil {
		return
	}
	log.Debugf("pubkey=%s", pubkey.String())

	resp = new(pbj.EndpointResponse)
	resp.Address = &pbj.Address{
		Port: 32,
		Ipv4: "1.23.3.3",
		Ipv6: "1243124",
	}
	resp.Certificate = []byte{}

	return
}
