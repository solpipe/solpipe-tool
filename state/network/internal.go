package network

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/state/slot"
	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
)

type internal struct {
	ctx              context.Context
	errorC           chan<- error
	closeSignalCList []chan<- error
	rpc              *sgorpc.Client
	slot             uint64
}

type BlockTransactionCount struct {
	Slot             uint64
	Fees             uint64
	TransactionCount uint64
	TransactionSize  uint64
	Time             time.Time
}

const DECIFER_GOROUTINE_COUNT = 5

func loopInternal(
	ctx context.Context,
	cancel context.CancelFunc,
	internalC <-chan func(*internal),
	rpcClient *sgorpc.Client,
	sub *sgows.BlockSubscription,
	slotHome slot.SlotHome,
	rawC chan<- *sgows.BlockResult,
) {
	var err error
	defer cancel()
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	in := new(internal)
	in.ctx = ctx
	in.errorC = errorC
	in.rpc = rpcClient
	in.slot = 0

	defer sub.Unsubscribe()
	streamBlockC := sub.RecvStream()
	errorBlockC := sub.CloseSignal()

	slotSub := slotHome.OnSlot()
	defer slotSub.Unsubscribe()
	voteDelay := 5 * time.Second
out:
	for {
		select {
		case <-time.After(voteDelay):
			voteDelay = 5 * time.Minute

		case err = <-slotSub.ErrorC:
			break out
		case err = <-errorBlockC:
			break out
		case d := <-streamBlockC:
			b, ok := d.(*sgows.BlockResult)
			if !ok {
				err = errors.New("bad block result")
				break out
			}
			if b.Value.Err != nil {
				err = fmt.Errorf("%+v", b.Value.Err)
				break out
			}
			rawC <- b
		case in.slot = <-slotSub.StreamC:
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Debug(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func loopCountBlock(
	ctx context.Context,
	cancel context.CancelFunc,
	rawC <-chan *sgows.BlockResult,
	blockCountHome *sub.SubHome[BlockTransactionCount],
) {
	doneC := ctx.Done()
	defer cancel()
out:
	for {
		select {
		case <-doneC:
			break out
		case id := <-blockCountHome.DeleteC:
			blockCountHome.Delete(id)
		case r := <-blockCountHome.ReqC:
			blockCountHome.Receive(r)
		case b := <-rawC:
			if b.Value.Err == nil {
				if b.Value.Block != nil {
					x := new(BlockTransactionCount)
					x.Slot = b.Value.Slot
					x.Fees = 0
					x.Time = b.Value.Block.BlockTime.Time()
					x.TransactionSize = 0
					x.TransactionCount = uint64(len(b.Value.Block.Transactions))
					for i := 0; i < len(b.Value.Block.Transactions); i++ {
						x.Fees += b.Value.Block.Transactions[i].Meta.Fee
						// hopefully there are not too many errors doing blocks
						tx, err := b.Value.Block.Transactions[i].GetTransaction()
						if err == nil {
							data, err := tx.MarshalBinary()
							if err == nil {
								x.TransactionSize += uint64(len(data))
							}
						}
					}
					blockCountHome.Broadcast(*x)
				}

			} else {
				log.Debugf("%+v", b.Value.Err)
			}

		}
	}
}
