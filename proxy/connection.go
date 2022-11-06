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
	pbj "github.com/solpipe/solpipe-tool/proto/job"
	"github.com/solpipe/solpipe-tool/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

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

	// the onion address goes here
	return grpc.DialContext(
		ctx,
		fmt.Sprintf("%s.onion:%d", onionID, util.DEFAULT_PROXY_PORT),
		grpc.WithTransportCredentials(credentials.NewTLS(getTlsConfig(
			y, destination,
		))),
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
		grpc.WithTransportCredentials(credentials.NewTLS(getTlsConfig(
			y, destination,
		)),
		),
		//grpc.WithBlock(),
	)
	if err != nil {
		return
	}

	go loopCloseConnection(ctx, conn)
	return
}

func getTlsConfig(cert *tls.Certificate, destination sgo.PublicKey) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{*cert},
		VerifyConnection: func(cs tls.ConnectionState) error {
			log.Debugf("client cs=%+v", cs)
			chain := cs.PeerCertificates
			if len(chain) != 2 {
				return errors.New("bad ca")
			}
			// leaf is ephemeral keypair
			certPool := x509.NewCertPool()
			certPool.AddCert(chain[1])
			_, err := chain[0].Verify(x509.VerifyOptions{
				Roots: certPool,
			})
			if err != nil {
				return err
			}
			if !VerifyCaWithx509(chain[1], destination) {
				return errors.New("destination pubkey does not match certificate")
			}
			return nil
		},
		InsecureSkipVerify: true,
		ServerName:         destination.String(),
	}
}

// connect via tor first, then see if there is a clear net connection
func CreateConnectionTorClearIfAvailable(
	ctx context.Context,
	destination sgo.PublicKey, // must be admin of Pipeline or Validator
	admin sgo.PrivateKey,
	torMgr *tor.Tor,
) (conn *grpc.ClientConn, err error) {
	log.Debug("attempting tor connection")
	c, err := CreateConnectionTor(ctx, destination, admin, torMgr)
	if err != nil {
		log.Debug("tor connection failed: %s", err.Error())
		return nil, err
	}

	log.Debug("tor connection successful, checking if clear net address exists")
	resp, err := pbj.NewEndpointClient(c).GetClearNetAddress(ctx, &pbj.EndpointRequest{})
	if err != nil {
		// we are stuck with tor since there is no clear net endpoint
		log.Debug("no clear net address exists")
		return c, nil
	}
	if resp == nil {
		log.Debug("no response")
		return c, nil
	}
	if resp.Address == nil {
		log.Debug("no response")
		return c, nil
	}
	if len(resp.Address.Ipv4) == 0 && len(resp.Address.Ipv6) == 0 {
		log.Debug("blank ipv4 and ipv6")
		return c, nil
	}

	// try ipv6
	if 0 < len(resp.Address.Ipv6) {
		log.Debug("attempting to ipv6 clear-net-connect to %s:%d", resp.Address.Ipv6, resp.Address.Port)
		newC, err := CreateConnectionClearNet(
			ctx,
			destination,
			fmt.Sprintf("%s:%d", resp.Address.Ipv6, resp.Address.Port),
			admin,
		)
		if err == nil {
			_, err = pbj.NewEndpointClient(newC).GetClearNetAddress(ctx, &pbj.EndpointRequest{})
			if err == nil {
				c.Close()
				log.Debug("returning clear-net connection")
				return newC, nil
			}
			log.Debug("failed to make request check")
		} else {
			log.Debugf("failed to do clear-net connection: %s", err.Error())
		}
	}
	{
		log.Debug("attempting to ipv4 clear-net-connect to %s:%d", resp.Address.Ipv4, resp.Address.Port)
		newC, err := CreateConnectionClearNet(
			ctx,
			destination,
			fmt.Sprintf("%s:%d", resp.Address.Ipv4, resp.Address.Port),
			admin,
		)
		if err == nil {
			_, err = pbj.NewEndpointClient(newC).GetClearNetAddress(ctx, &pbj.EndpointRequest{})
			if err == nil {
				c.Close()
				return newC, nil
			}
		} else {
			log.Debug(err)
		}
	}
	return c, nil
}
