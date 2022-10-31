package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"time"

	_ "embed"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func CreateListener(
	ctx context.Context,
	admin sgo.PrivateKey,
) (s *grpc.Server, err error) {

	var y *tls.Certificate
	y, err = NewSelfSignedTlsCertificateChainServer(
		admin,
		[]string{"blank"},
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		grpc.WithReturnConnectionError()
	}

	verifyConnection := func(cs tls.ConnectionState) error {
		log.Debugf("server cs=%+v", cs)

		chain := cs.PeerCertificates
		if len(chain) != 2 {
			return errors.New("bad ca")
		}
		// leaf is ephemeral keypair
		certPool := x509.NewCertPool()
		certPool.AddCert(chain[1])
		_, err2 := chain[0].Verify(x509.VerifyOptions{
			Roots: certPool,
		})
		if err2 != nil {
			return err2
		}
		return nil
	}

	config := &tls.Config{
		Certificates:     []tls.Certificate{*y},
		VerifyConnection: verifyConnection,
		ClientAuth:       tls.RequireAnyClientCert,
	}

	s = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(config)),
		grpc.UnaryInterceptor(func(ctx2 context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			log.Debugf("server unary=%+v", info)
			return handler(ctx2, req)
		}),
		grpc.StreamInterceptor(func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			log.Debugf("server stream=%+v", info)
			return handler(srv, ss)
		}),
	)

	return
}

type ListenerInfo struct {
	Addresses []string
	Listener  net.Listener
}

func CreateListenerClearNet(
	ctx context.Context,
	listenUrl string,
	routableAddresses []string,
) (li *ListenerInfo, err error) {
	li = new(ListenerInfo)
	li.Addresses = routableAddresses
	li.Listener, err = net.Listen("tcp", listenUrl)
	return
}

func CreateListenerTor(
	ctx context.Context,
	admin sgo.PrivateKey,
	t *tor.Tor,
) (li *ListenerInfo, err error) {

	priv := ed25519.PrivateKey(admin)
	var address string
	address, err = util.GenerateOnionAddressFromSolanaPublicKey(admin.PublicKey())
	if err != nil {
		return
	}
	var onion *tor.OnionService
	onion, err = t.Listen(
		ctx,
		&tor.ListenConf{
			Version3:    true,
			RemotePorts: []int{util.DEFAULT_PROXY_PORT},
			LocalPort:   0,
			Key:         priv,
		},
	)
	if err != nil {
		return
	}
	li = new(ListenerInfo)
	li.Listener = onion

	if onion.ID != address {
		err = fmt.Errorf("bad onion address; \n%s\n%s", onion.ID, address)
		return
	}
	li.Addresses = []string{fmt.Sprintf("%s.onion:%d", onion.ID, util.DEFAULT_PROXY_PORT)}
	return
}
