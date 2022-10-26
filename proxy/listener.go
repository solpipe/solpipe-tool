package proxy

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"

	_ "embed"

	"github.com/solpipe/solpipe-tool/util"
	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
)

func CreateListener(
	ctx context.Context,
	admin sgo.PrivateKey,
	config *ServerConfiguration,
	t *tor.Tor,
) (l net.Listener, err error) {
	if config == nil {
		//err = errors.New("no config")
		//return
		config = &ServerConfiguration{}
	}

	if t == nil {
		err = errors.New("not implemented yet")
		return
	} else {
		priv := ed25519.PrivateKey(admin)
		var address string
		address, err = util.GenerateOnionAddressFromSolanaPublicKey(admin.PublicKey())
		if err != nil {
			return
		}
		var onion *tor.OnionService
		log.Debugf("attempting to listen on tor onion address for listener admin=%s", admin.PublicKey().String())
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
		log.Debugf("listener admin=%s listening on onion=%s.onion:%d", admin.PublicKey().String(), onion.ID, util.DEFAULT_PROXY_PORT)

	}
	return
}
