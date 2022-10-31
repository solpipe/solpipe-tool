package proxy_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/proxy"
	"github.com/solpipe/solpipe-tool/proxy/relay"
	"google.golang.org/grpc"
)

func TestTor(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		time.Sleep(30 * time.Second)
	})
	{
		closeListener, err := net.Listen("tcp", ":3003")
		if err != nil {
			return
		}
		cancelCopy := cancel
		go func() {
			closeListener.Accept()
			cancelCopy()
		}()
		doneC := ctx.Done()
		go func() {
			<-doneC
			closeListener.Close()
		}()
	}
	pipeline, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	bidder, err := sgo.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	errorC := make(chan error, 1)

	{
		torMgr, err := proxy.SetupTor(ctx, false)
		if err != nil {
			t.Fatal(err)
		}

		innerListener, err := proxy.CreateListenerTor(
			ctx,
			pipeline,
			torMgr,
		)
		if err != nil {
			t.Fatal(err)
		}

		var s *grpc.Server
		s, err = proxy.CreateListener(
			ctx,
			pipeline,
			innerListener,
		)
		if err != nil {
			t.Fatal(err)
		}

		attach(ctx, s)

		go loopListenClose(ctx, innerListener.Listener)
		go loopListen(errorC, innerListener.Listener, s)

	}

	go func() {
		torMgr, err := proxy.SetupTor(ctx, false)
		if err != nil {
			errorC <- err
			return
		}
		time.Sleep(2 * time.Minute)
		var conn *grpc.ClientConn
		log.Debug("connecting to tor")
		conn, err = proxy.CreateConnectionTor(
			ctx,
			pipeline.PublicKey(),
			bidder,
			torMgr,
		)
		if err != nil {
			errorC <- err
			return
		}

		time.Sleep(2 * time.Minute)
		client := pbj.NewEndpointClient(conn)
		log.Debug("starting client")
		//time.Sleep(10 * time.Minute)
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
		cancel()
	}()

	select {
	case <-ctx.Done():
	case err = <-errorC:
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestClearNet(t *testing.T) {
	log.SetLevel(log.DebugLevel)
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

		var s *grpc.Server
		s, err = proxy.CreateListener(
			ctx,
			pipeline,
			innerListener,
		)
		if err != nil {
			t.Fatal(err)
		}

		attach(ctx, s)
		l = innerListener.Listener
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
		//time.Sleep(10 * time.Minute)
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
		cancel()
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
	log.Debugf("ctx=%+v", ctx)
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
