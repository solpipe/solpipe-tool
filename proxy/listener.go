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
)

func CreateListener(
	ctx context.Context,
	admin sgo.PrivateKey,
	innerListener *ListenerInfo,
) (l net.Listener, err error) {

	if innerListener == nil {
		err = errors.New("no listener")
		return
	}

	var x *TlsForServer
	x, err = NewSelfSignedTlsCertificateChainServer(
		admin,
		innerListener.Addresses,
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		return nil, err
	}
	var y tls.Certificate
	y, err = x.CertAndKey()
	if err != nil {
		return nil, err
	}

	l = tls.NewListener(
		innerListener.Listener,
		&tls.Config{
			Certificates: []tls.Certificate{y},
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				// return always true
				// we assume that the self-signed certificates have valid signatures
				// and that the PublicKey will return a valid public key
				return nil
			},
			VerifyConnection: func(cs tls.ConnectionState) error {
				log.Debugf("cs=%+v", cs)
				return nil
			},
			ClientAuth: tls.RequireAnyClientCert,
		},
	)

	return
}

type ListenerInfo struct {
	Addresses []string
	Listener  net.Listener
}

func CreateListenerLocal(
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
	log.Debugf("+++++attempting to listen on tor onion address for listener admin=%s", admin.PublicKey().String())

	//ctxC, cancel := context.WithTimeout(ctx, 1*time.Minute)
	//defer cancel()
	onion, err = t.Listen(
		ctx,
		&tor.ListenConf{
			Version3:    true,
			RemotePorts: []int{util.DEFAULT_PROXY_PORT},
			LocalPort:   0,
			Key:         priv,
			//NonAnonymous: true,
		},
	)
	if err != nil {
		return
	}
	li = new(ListenerInfo)
	li.Listener = onion
	// Check in the future
	if onion.ID != address {
		err = fmt.Errorf("bad onion address; \n%s\n%s", onion.ID, address)
		return
	}
	log.Debugf("++++listener admin=%s listening on onion=%s.onion:%d", admin.PublicKey().String(), onion.ID, util.DEFAULT_PROXY_PORT)
	li.Addresses = []string{fmt.Sprintf("%s.onion:%d", onion.ID, util.DEFAULT_PROXY_PORT)}
	return
}

func CreateListenerDeprecated(
	ctx context.Context,
	admin sgo.PrivateKey,
	config *ServerConfiguration,
	t *tor.Tor,
) (l net.Listener, err error) {

	if t == nil {
		if config == nil {
			err = errors.New("no config")
			return
		}
		var x *TlsForServer
		x, err = NewSelfSignedTlsCertificateChainServer(
			admin,
			config.dns_string(),
			time.Now().Add(7*24*time.Hour),
		)
		if err != nil {
			return nil, err
		}
		var y tls.Certificate
		y, err = x.CertAndKey()
		if err != nil {
			return nil, err
		}

		return tls.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.Port), &tls.Config{
			Certificates: []tls.Certificate{y},
		})
		//l, err = net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.Port))
	} else {
		priv := ed25519.PrivateKey(admin)
		var address string
		address, err = util.GenerateOnionAddressFromSolanaPublicKey(admin.PublicKey())
		if err != nil {
			return
		}
		var onion *tor.OnionService
		log.Debugf("+++++attempting to listen on tor onion address for listener admin=%s", admin.PublicKey().String())

		//ctxC, cancel := context.WithTimeout(ctx, 1*time.Minute)
		//defer cancel()
		onion, err = t.Listen(
			ctx,
			&tor.ListenConf{
				Version3:    true,
				RemotePorts: []int{util.DEFAULT_PROXY_PORT},
				LocalPort:   0,
				Key:         priv,
				//NonAnonymous: true,
			},
		)
		if err != nil {
			return
		}

		l = onion
		// Check in the future
		if onion.ID != address {
			err = fmt.Errorf("bad onion address; \n%s\n%s", onion.ID, address)
			return
		}
		log.Debugf("++++listener admin=%s listening on onion=%s.onion:%d", admin.PublicKey().String(), onion.ID, util.DEFAULT_PROXY_PORT)

	}
	return
}
