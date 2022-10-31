package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func CreateConnectionToPipelineTor(
	ctx context.Context,
	pipeline pipe.Pipeline,
	t1 *tor.Tor,
) (conn *grpc.ClientConn, err error) {
	if t1 == nil {
		err = errors.New("non-tor connections are not supported")
		return
	}
	log.Debugf("connecting over tor to pipeline=%s", pipeline.Id.String())
	var dialer *tor.Dialer
	dialer, err = t1.Dialer(ctx, nil)
	if err != nil {
		return
	}
	ctxC, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	conn, err = pipeline.Dial(ctxC, dialer)
	if err != nil {
		return
	}
	go loopCloseConnection(ctx, conn)
	return
}

func loopCloseConnection(ctx context.Context, conn *grpc.ClientConn) {
	<-ctx.Done()
	conn.Close()
}

func CreateConnectionTor(
	ctx context.Context,
	destination sgo.PublicKey, // must be admin of Pipeline or Validator
	admin sgo.PrivateKey,
	torMgr *tor.Tor,
) (conn *grpc.ClientConn, err error) {

	ctxC, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	var dialer *tor.Dialer
	dialer, err = torMgr.Dialer(ctxC, nil)
	if err != nil {
		return
	}
	onionID, err := util.GetOnionID(destination.Bytes())
	if err != nil {
		return nil, err
	}
	log.Debugf("from client; destination=%s onion id=%s.onion:%d", destination.String(), onionID, util.DEFAULT_PROXY_PORT)

	var y *tls.Certificate
	y, err = NewSelfSignedTlsCertificateChainServer(
		admin,
		[]string{"client"},
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		return
	}
	conifg := &tls.Config{
		Certificates: []tls.Certificate{*y},
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return verify(destination, rawCerts, verifiedChains)
		},
	}

	// the onion address goes here
	return grpc.DialContext(
		ctx,
		fmt.Sprintf("%s.onion:%d", onionID, util.DEFAULT_PROXY_PORT),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(credentials.NewTLS(conifg)),
		grpc.WithContextDialer(func(ctxInside context.Context, addr string) (net.Conn, error) {
			return dialer.DialContext(ctxInside, "tcp", addr)
		}),
	)
}

func CreateConnectionClearNet(
	ctx context.Context,
	destination sgo.PublicKey,
	destinationUrl string,
	admin sgo.PrivateKey,
) (conn *grpc.ClientConn, err error) {

	ctxC, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	var y *tls.Certificate
	y, err = NewSelfSignedTlsCertificateChainServer(
		admin,
		[]string{"client"},
		time.Now().Add(7*24*time.Hour),
	)

	conn, err = grpc.DialContext(
		ctxC,
		destinationUrl,
		//grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{*y},
			VerifyConnection: func(cs tls.ConnectionState) error {
				log.Debugf("client cs=%+v", cs)
				return nil
				//return errors.New("stop me")
			},
			InsecureSkipVerify: true,
			// very ugly hack so that the server will see what root CA to add for this connection
			//ServerName: string(serializeCertDerToPem(y.Certificate[1])),
			ServerName: destination.String(),
		}),
		),
		//grpc.WithBlock(),
	)
	if err != nil {
		return
	}

	go loopCloseConnection(ctx, conn)
	return
}
