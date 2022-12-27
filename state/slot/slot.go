package slot

import (
	"context"
	"errors"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
)

type SlotHome struct {
	id         uuid.UUID
	reqC       chan<- dssub.ResponseChannel[uint64]
	ctx        context.Context
	singleReqC chan<- chan<- uint64
}

func SubscribeSlot(
	ctxOutside context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
) (SlotHome, error) {
	ctx, cancel := context.WithCancel(ctxOutside)

	home := dssub.CreateSubHome[uint64]()
	reqC := home.ReqC
	var err error
	id, err := uuid.NewRandom()
	if err != nil {
		cancel()
		return SlotHome{}, err
	}

	singleReqC := make(chan chan<- uint64)
	go loopInternal(
		ctx,
		home,
		rpcClient,
		wsClient,
		cancel,
		id,
		singleReqC,
	)

	//log.Debugf("creating slot subber id=%s", id.String())
	return SlotHome{
		reqC: reqC, ctx: ctx, id: id, singleReqC: singleReqC,
	}, nil
}

func (sh SlotHome) Time() (uint64, error) {
	err := sh.ctx.Err()
	if err != nil {
		return 0, err
	}
	doneC := sh.ctx.Done()
	respC := make(chan uint64, 1)
	select {
	case <-doneC:
		return 0, errors.New("canceled")
	case sh.singleReqC <- respC:
	}
	select {
	case <-doneC:
		return 0, errors.New("canceled")
	case ans := <-respC:
		return ans, nil
	}
}

const SLOT_BUFFER_SIZE = 100

func allowAllSlot(x uint64) bool {
	return true
}

func (sh SlotHome) OnSlot() dssub.Subscription[uint64] {
	return dssub.SubscriptionRequestWithBufferSize(sh.reqC, SLOT_BUFFER_SIZE, allowAllSlot)
}

func (sh SlotHome) CloseSignal() <-chan struct{} {
	return sh.ctx.Done()
}

// carry websocket client into this goroutine to prevent it from going out of memory and killing the subscriptions
func loopInternal(
	ctx context.Context,
	home *dssub.SubHome[uint64],
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
	cancel context.CancelFunc,
	id uuid.UUID,
	singleReqC <-chan chan<- uint64,
) {
	var err error
	var sub *sgows.SlotSubscription
	defer cancel()
	doneC := ctx.Done()
	reqC := home.ReqC
	deleteC := home.DeleteC
	defer home.Close()

	sub, err = wsClient.SlotSubscribe()
	if err != nil {
		log.Debug(err)
		return
	}
	streamC := sub.RecvStream()
	streamErrorC := sub.CloseSignal()
	defer sub.Unsubscribe()

	log.Debug("preparing slot stream")
	time.Sleep(5 * time.Second)
	interval := 3 * time.Second
	nextC := time.After(interval)
	lastSlot := uint64(0)
	slot := uint64(0)

out:
	for {
		select {
		case <-nextC:
			slot, err = rpcClient.GetSlot(ctx, sgorpc.CommitmentFinalized)
			if err != nil {
				break out
			}
			sendBroadcat(home, &lastSlot, &slot)
			lastSlot = slot
			nextC = time.After(interval)
		case respC := <-singleReqC:
			respC <- slot
		case d := <-streamC:
			x, ok := d.(*sgows.SlotResult)
			if !ok {
				break out
			}
			slot = x.Slot
			if slot%50 == 0 {
				log.Debugf("slothome slot=%d", slot)
			}
			sendBroadcat(home, &lastSlot, &slot)
			lastSlot = slot
			nextC = time.After(interval)
		case err = <-streamErrorC:
			if err != nil {
				log.Debugf("slot stream error: %s", err.Error())
				break out
			}
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
	if *slot%50 == 0 {
		log.Debugf("slot____=%d; sub count=%d", *slot, home.SubscriberCount())
	}
	//log.Debugf("slot____=%d; sub count=%d", *slot, home.SubscriberCount())
	*lastSlot = *slot
	home.Broadcast(*slot)
}
