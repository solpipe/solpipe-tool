package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// get the public key from the peer connection
func GetPeerPubkey(ctx context.Context) (pubkey sgo.PublicKey, err error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		err = errors.New("no peer certificate")
		return
	}
	if p.AuthInfo == nil {
		err = errors.New("auth info is nil")
		return
	}
	tlsInfo := p.AuthInfo.(credentials.TLSInfo)
	if len(tlsInfo.State.PeerCertificates) != 1 {
		err = errors.New("do not have single certificate")
		return
	}
	peerCert := tlsInfo.State.PeerCertificates[0]

	if peerCert.PublicKeyAlgorithm != x509.Ed25519 {
		err = errors.New("wrong public key algorithm")
	}
	var pubkeyPre ed25519.PublicKey

	pubkeyPre, ok = peerCert.PublicKey.(ed25519.PublicKey)
	if !ok {
		err = errors.New("failed to get public key")
		return
	}

	pubkey = sgo.PublicKeyFromBytes(pubkeyPre)

	return
}
