package receipt

import (
	"context"

	cba "github.com/solpipe/cba"
	sch "github.com/solpipe/solpipe-tool/scheduler"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
)

type txInfo struct {
	cancelApprove context.CancelFunc
}

func loopUpdate(
	ctx context.Context,
	rwd rpt.ReceiptWithData,
	errorC chan<- error,
	eventC chan<- sch.Event,
) {
	var err error
	doneC := ctx.Done()

	ti := new(txInfo)

	dataSub := rwd.Receipt.OnData()
	defer dataSub.Unsubscribe()
	data, err := rwd.Receipt.Data()
	if err != nil {
		select {
		case <-doneC:
		case errorC <- err:
		}
		return
	}
	lastTxSent := data.TxSent
	if 0 < lastTxSent && data.TxSent != data.ValidatorSigned {
		select {
		case <-doneC:
		case eventC <- sch.Create(
			sch.EVENT_RECEIPT_NEW_COUNT,
			false,
			0,
		):
		}
	} else if 0 < lastTxSent && data.TxSent == data.ValidatorSigned {
		select {
		case <-doneC:
		case eventC <- sch.Create(
			sch.EVENT_RECEIPT_APPROVED,
			false,
			0,
		):
		}
	}

out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case data = <-dataSub.StreamC:
			if lastTxSent < data.TxSent && data.TxSent != data.ValidatorSigned {
				if ti.cancelApprove != nil {
					ti.cancelApprove()
					ti.cancelApprove = nil
				}
				var ctxApprove context.Context
				ctxApprove, ti.cancelApprove = context.WithCancel(ctx)
				select {
				case <-doneC:
					ti.cancelApprove()
				case eventC <- sch.CreateWithPayload(
					sch.TRIGGER_RECEIPT_APPROVE,
					true,
					0,
					&Trigger{
						Context: ctxApprove,
						Receipt: rwd.Receipt,
						Data:    data,
					},
				):
				}
			} else if lastTxSent <= data.TxSent && data.TxSent == data.ValidatorSigned {
				if ti.cancelApprove != nil {
					ti.cancelApprove()
					ti.cancelApprove = nil
				}
				select {
				case <-doneC:
				case eventC <- sch.Create(
					sch.EVENT_RECEIPT_APPROVED,
					true,
					0,
				):
				}
			}
			lastTxSent = data.TxSent

		}
	}

	if err != nil {
		select {
		case errorC <- err:
		default:
		}
	}
}

type Trigger struct {
	Context context.Context
	Receipt rpt.Receipt
	Data    cba.Receipt
}
