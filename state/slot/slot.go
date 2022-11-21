package slot

import (
	"context"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
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

func SubscribeSlot(ctxOutside context.Context, rpcClient *sgorpc.Client, wsClient *sgows.Client) (SlotHome, error) {
	ctx, cancel := context.WithCancel(ctxOutside)

	home := dssub.CreateSubHome[uint64]()
	reqC := home.ReqC
	var err error
	id, err := uuid.NewRandom()
	if err != nil {
		cancel()
		return SlotHome{}, err
	}
	sub, err := wsClient.SlotSubscribe()
	if err != nil {
		cancel()
		return SlotHome{}, err
	}
	go loopSlot(ctx, home, rpcClient, sub, cancel, id)

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
	home *dssub.SubHome[uint64],
	rpcClient *sgorpc.Client,
	sub *sgows.SlotSubscription,
	cancel context.CancelFunc,
	id uuid.UUID,
) {
	var err error
	defer cancel()
	doneC := ctx.Done()
	reqC := home.ReqC
	deleteC := home.DeleteC

	streamC := sub.RecvStream()
	closeC := sub.CloseSignal()
	defer sub.Unsubscribe()

	log.Debug("preparing slot stream")
	time.Sleep(5 * time.Second)
	interval := 3 * time.Second
	nextC := time.After(interval)
	var lastSlot uint64
	lastSlot = 0
	var slot uint64
out:
	for {
		select {
		case d := <-streamC:
			x, ok := d.(*sgows.SlotResult)
			if !ok {
				break out
			}
			slot = x.Slot
			sendBroadcat(home, &lastSlot, &slot)
			lastSlot = slot
			nextC = time.After(interval)
		case <-nextC:
			slot, err = rpcClient.GetSlot(ctx, sgorpc.CommitmentFinalized)
			if err != nil {
				break out
			}
			sendBroadcat(home, &lastSlot, &slot)
			lastSlot = slot
			nextC = time.After(interval)
		case <-closeC:
			break out
		case <-doneC:
			break out
		case rC := <-reqC:
			log.Debugf("slot subscription (%s)  %d", id.String(), home.SubscriberCount())
			home.Receive(rC)
		case id := <-deleteC:
			home.Delete(id)
		}
	}

	log.Debug("++++++++++++++++++++++++++++exiting slot subscription+++++++++++++++++++++++++++")
	log.Debug(err)
}

func sendBroadcat(
	home *dssub.SubHome[uint64],
	lastSlot *uint64,
	slot *uint64,
) {
	if *slot <= *lastSlot {
		return
	}
	if *slot%500 == 0 {
		log.Debugf("slot____=%d; sub count=%d", *slot, home.SubscriberCount())
	}
	//log.Debugf("slot____=%d; sub count=%d", *slot, home.SubscriberCount())
	*lastSlot = *slot
	home.Broadcast(*slot)
}
