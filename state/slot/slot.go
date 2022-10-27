package slot

import (
	"context"

	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
)

type SlotHome struct {
	id   uuid.UUID
	reqC chan<- dssub.ResponseChannel[uint64]
	ctx  context.Context
}

func SubscribeSlot(ctxOutside context.Context, wsClient *sgows.Client) (SlotHome, error) {
	ctx, cancel := context.WithCancel(ctxOutside)
	initC := make(chan error, 1)
	home := dssub.CreateSubHome[uint64]()
	reqC := home.ReqC
	var err error
	id, err := uuid.NewRandom()
	if err != nil {
		cancel()
		return SlotHome{}, err
	}

	go loopSlot(ctx, initC, home, wsClient, cancel, id)
	err = <-initC
	if err != nil {
		return SlotHome{}, err
	}
	//log.Debugf("creating slot subber id=%s", id.String())
	return SlotHome{
		reqC: reqC, ctx: ctx, id: id,
	}, nil
}

func (sh SlotHome) OnSlot() dssub.Subscription[uint64] {
	return dssub.SubscriptionRequest(sh.reqC, func(x uint64) bool { return true })
}

func (sh SlotHome) CloseSignal() <-chan struct{} {
	return sh.ctx.Done()
}

// carry websocket client into this goroutine to prevent it from going out of memory and killing the subscriptions
func loopSlot(
	ctx context.Context,
	initErrorC chan<- error,
	home *dssub.SubHome[uint64],
	wsClient *sgows.Client,
	cancel context.CancelFunc,
	id uuid.UUID,
) {
	var err error
	defer cancel()
	doneC := ctx.Done()
	reqC := home.ReqC
	deleteC := home.DeleteC

	sub, err := wsClient.SlotSubscribe()
	initErrorC <- err
	if err != nil {
		return
	}

	streamC := sub.RecvStream()
	closeC := sub.CloseSignal()
	defer sub.Unsubscribe()

out:
	for {
		select {
		case d := <-streamC:
			x, ok := d.(*sgows.SlotResult)
			if !ok {
				break out
			}
			//if x.Slot%10 == 0 {
			//	log.Debugf("(%s)slot____=%d; sub count=%d", id.String(), x.Slot, home.SubscriberCount())
			//}
			home.Broadcast(x.Slot)
		case <-closeC:
			break out
		case <-doneC:
			break out
		case rC := <-reqC:
			//log.Debugf("slot subscription (%s)  %d", id.String(), home.SubscriberCount())
			home.Receive(rC)
		case id := <-deleteC:
			home.Delete(id)
		}
	}

	log.Debug("exiting slot subscription")
	log.Debug(err)
}
