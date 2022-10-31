package relay

import (
	"context"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/solpipe/solpipe-tool/proxy"
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
	if len(tlsInfo.State.PeerCertificates) != 2 {
		err = errors.New("do not have sufficient certificates")
		return
	}

	pubkey, err = proxy.PubkeyFromCaX509(tlsInfo.State.PeerCertificates[1])
	if err != nil {
		return
	}

	return
}
