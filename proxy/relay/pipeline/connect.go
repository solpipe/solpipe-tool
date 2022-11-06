package pipeline

import (
	"context"
	"time"

	sgo "github.com/SolmateDev/solana-go"
	"github.com/cretz/bine/tor"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/proxy"
	pxyclt "github.com/solpipe/solpipe-tool/proxy/client"
	"github.com/solpipe/solpipe-tool/script"
	val "github.com/solpipe/solpipe-tool/state/validator"
	"google.golang.org/grpc"
)

type validatorClientWithId struct {
	id     sgo.PublicKey
	client pxyclt.Client
}

// make sure we are connected
func loopValidatorConnect(
	ctx context.Context,
	cancel context.CancelFunc,
	torMgr *tor.Tor,
	errorC chan<- error,
	pleaseConnectC <-chan struct{},
	connC chan<- validatorClientWithId,
	validator val.Validator,
	data cba.ValidatorManager,
	admin sgo.PrivateKey,
	scriptBuidler *script.Script,
) {
	defer cancel()
	pipelineSender := admin
	destinationValidator := data.Admin

	var conn *grpc.ClientConn
	var client pxyclt.Client

	doneC := ctx.Done()
	var err error

	internalPleaseConnectC := make(chan struct{}, 1)
	delay := 1 * time.Minute
out:
	for {
		select {
		case <-doneC:
			break out
		case <-internalPleaseConnectC:
			log.Debug("connecting from relay-pipeline to validator")
			conn, err = proxy.CreateConnectionTorClearIfAvailable(
				ctx,
				destinationValidator,
				pipelineSender,
				torMgr,
			)
			if err != nil {
				log.Debugf("relay-pipeline failed to connect: %s", err.Error())
				go loopDelayConnect(ctx, delay, internalPleaseConnectC)
				err = nil
			} else {
				log.Debug("relay-pipeline successfully connected")
				client, err = pxyclt.Create(
					ctx,
					conn,
					nil,
					admin,
					data.Admin,
					nil,
					&script.ReceiptSettings{},
					scriptBuidler,
				)
				if err != nil {
					break out
				}
				connC <- validatorClientWithId{
					id:     validator.Id,
					client: client,
				}
			}
		case <-pleaseConnectC:
			internalPleaseConnectC <- struct{}{}
		}
	}
	if err != nil {
		errorC <- err
	}
}

func loopDelayConnect(ctx context.Context, delay time.Duration, pleaseConnectC chan<- struct{}) {
	doneC := ctx.Done()
	select {
	case <-doneC:
	case <-time.After(delay):
		select {
		case <-doneC:
		case pleaseConnectC <- struct{}{}:
		}
	}
}
