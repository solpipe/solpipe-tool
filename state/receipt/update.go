package receipt

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	cba "github.com/solpipe/cba"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
)

func (e1 Receipt) Update(data cba.Receipt) {
	e1.dataC <- data
	return
}

func (in *internal) on_data(data cba.Receipt) {
	in.receiptHome.Broadcast(data)
	in.data = &data
}

// Staker will delete itself if the stakemember account is closed.
func (e1 Receipt) UpdateStaker(s skr.Staker) {
	// stakers will not change from receipt
	e1.internalC <- func(in *internal) {
		in.on_staker(s)
	}
}

func (in *internal) on_staker(s skr.Staker) {

	_, present := in.stakers[s.Id.String()]
	if present {
		return
	}

	in.stakers[s.Id.String()] = s
	si := new(stakerInternal)
	si.ctx = in.ctx
	si.errorC = in.errorC
	si.updateC = in.updateStakerManagerC
	go si.loop(s)
}

type stakerInternal struct {
	ctx     context.Context
	errorC  chan<- error
	updateC chan<- sub.StakeGroup
	deleteC chan<- sgo.PublicKey
}

// TODO: validator and payout will subscribe
func (si *stakerInternal) loop(s skr.Staker) {
	id := s.Id
	doneC := si.ctx.Done()
	closeC := s.OnClose()
	stakeSub := s.OnData()
	defer stakeSub.Unsubscribe()

	var err error
out:
	for {
		select {
		case <-doneC:
			break out
		case <-closeC:
			break out
		case err = <-stakeSub.ErrorC:
			break out
		case d := <-stakeSub.StreamC:
			si.updateC <- sub.StakeGroup{
				Id:   id,
				Data: d,
			}

		}
	}

	si.deleteC <- id

	if err != nil {
		si.errorC <- err
	}

}
