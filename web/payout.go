package web

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	pyt "github.com/solpipe/solpipe-tool/state/payout"
	"github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
	"github.com/solpipe/solpipe-tool/util"
)

func (e1 external) ws_payout(
	clientCtx context.Context,
	errorC chan<- error,
	pOut payoutChannelGroup,
) {

	list, err := e1.router.AllPipeline()
	if err != nil {
		errorC <- err
		return
	}
	for _, x := range list {
		pwdList, err := x.AllPayouts()
		if err != nil {
			errorC <- err
			return
		}
		for _, y := range pwdList {
			go e1.ws_on_payout(
				errorC,
				clientCtx,
				y.Payout,
				pOut,
			)
			err = pOut.writeData(sub.PayoutWithData{
				Id:     y.Id,
				Data:   y.Data,
				IsOpen: true,
			})
			if err != nil {
				errorC <- err
				return
			}
		}

	}
}

func (e1 external) ws_on_payout(
	errorC chan<- error,
	ctx context.Context,
	p pyt.Payout,
	pOut payoutChannelGroup,
) {
	var err error
	serverDoneC := e1.ctx.Done()
	doneC := ctx.Done()

	id := p.Id

	dataSub := p.OnData()
	defer dataSub.Unsubscribe()

	{
		d, err := p.Data()
		if err != nil {
			errorC <- err
			return
		}
		err = pOut.writeData(sub.PayoutWithData{
			Id:     id,
			Data:   d,
			IsOpen: true,
		})
		if err != nil {
			errorC <- err
			return
		}
	}

	{
		list, err := p.Receipt()
		if err != nil {
			errorC <- err
			return
		}
		for _, rwd := range list {

			err = pOut.writeReceipt(sub.ReceiptGroup{
				Id:     rwd.Receipt.Id,
				Data:   rwd.Data,
				IsOpen: true,
			})
			if err != nil {
				errorC <- err
				return
			}
			go e1.ws_on_receipt(errorC, rwd, pOut)
		}
	}
	receiptSub := p.OnReceipt(util.Zero())
	defer receiptSub.Unsubscribe()

out:
	for {
		select {
		case <-serverDoneC:
			break out
		case <-doneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case d := <-dataSub.StreamC:
			err = pOut.writeData(sub.PayoutWithData{
				Id:     id,
				Data:   d,
				IsOpen: true,
			})
		case err = <-receiptSub.ErrorC:
			break out
		case rwd := <-receiptSub.StreamC:
			go e1.ws_on_receipt(errorC, rwd, pOut)
		}
	}
	if err != nil {
		errorC <- err
	} else {
		err = pOut.writeData(sub.PayoutWithData{
			Id:     id,
			IsOpen: false,
		})
		if err != nil {
			errorC <- err
		}
	}
}

func (e1 external) ws_on_receipt(
	errorC chan<- error,
	rwd receipt.ReceiptWithData,
	pOut payoutChannelGroup,
) {
	var err error
	id := rwd.Receipt.Id
	serverDoneC := e1.ctx.Done()
	doneC := pOut.clientCtx.Done()

	dataSub := rwd.Receipt.OnData()
	defer dataSub.Unsubscribe()

out:

	for {
		select {
		case <-doneC:
			break out
		case <-serverDoneC:
			break out
		case err = <-dataSub.ErrorC:
			break out
		case d := <-dataSub.StreamC:
			pOut.writeReceipt(sub.ReceiptGroup{
				Id:     id,
				Data:   d,
				IsOpen: true,
			})
		}
	}

	if err != nil {
		log.Debugf("closing payout=%s due to error: %s", id.String(), err.Error())
		errorC <- err
	} else {
		log.Debugf("sending open=false to payout=%s", id.String())
		err = pOut.writeReceipt(sub.ReceiptGroup{
			Id:     id,
			IsOpen: false,
		})
		if err != nil {
			errorC <- err
		}
	}

}

type payoutChannelGroup struct {
	serverCtx context.Context
	clientCtx context.Context
	dataC     chan<- sub.PayoutWithData
	receiptC  chan<- sub.ReceiptGroup
}

func (out payoutChannelGroup) writeData(d sub.PayoutWithData) error {
	var err error
	select {
	case <-out.serverCtx.Done():
		err = errors.New("canceled")
	case <-out.clientCtx.Done():
		err = errors.New("canceled")
	case out.dataC <- d:
	}
	return err
}

func (out payoutChannelGroup) writeReceipt(r sub.ReceiptGroup) error {
	var err error
	select {
	case <-out.serverCtx.Done():
		err = errors.New("canceled")
	case <-out.clientCtx.Done():
		err = errors.New("canceled")
	case out.receiptC <- r:
	}
	return err
}

type payoutChannelGroupInternal struct {
	serverCtx context.Context
	clientCtx context.Context
	dataC     <-chan sub.PayoutWithData
	receiptC  <-chan sub.ReceiptGroup
}

func (e1 external) createPayoutPair(
	clientCtx context.Context,
) (payoutChannelGroup, payoutChannelGroupInternal) {
	dataC := make(chan sub.PayoutWithData)
	receiptC := make(chan sub.ReceiptGroup)

	return payoutChannelGroup{
			clientCtx: clientCtx,
			serverCtx: e1.ctx,
			dataC:     dataC,
			receiptC:  receiptC,
		},
		payoutChannelGroupInternal{
			clientCtx: clientCtx,
			serverCtx: e1.ctx,
			dataC:     dataC,
			receiptC:  receiptC,
		}
}
