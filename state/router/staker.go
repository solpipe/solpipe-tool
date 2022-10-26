package router

import (
	"errors"

	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
	sgo "github.com/SolmateDev/solana-go"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
)

type refStaker struct {
	s skr.Staker
}

type lookUpStaker struct {
	byId              map[string]*refStaker
	byStake           map[string]*refStaker
	delegationStreamC <-chan sgows.Result
	delegationErrorC  <-chan error
}

func (in *internal) createLookupStaker() (*lookUpStaker, error) {

	return &lookUpStaker{
		byId:    make(map[string]*refStaker),
		byStake: make(map[string]*refStaker),
	}, nil
}

func (in *internal) on_stake(obj sub.StakeGroup) error {
	id := obj.Id
	if !obj.IsOpen {
		y, present := in.l_staker.byId[id.String()]
		if present {
			y.s.Close()
			delete(in.l_staker.byId, id.String())
		}
		return nil
	}

	data := obj.Data
	var err error

	newlyCreated := false
	ref, present := in.l_staker.byId[id.String()]
	if !present {
		ref = new(refStaker)
		ref.s, err = skr.CreateStake(in.ctx, obj)
		if err != nil {
			return err
		}
		newlyCreated = true
		in.l_staker.byId[data.Receipt.String()] = ref
		in.l_staker.byStake[data.Stake.String()] = ref
	} else {
		ref.s.Update(obj)
	}

	// look up receipt
	// data.Receipt
	r, present := in.l_receipt.byId[data.Receipt.String()]
	if present {
		r.UpdateStaker(ref.s)
	} else {
		x, present := in.l_receipt.stakerWithNoReceipt[r.Id.String()]
		if !present {
			x = make(map[string]skr.Staker)
			in.l_receipt.stakerWithNoReceipt[r.Id.String()] = x
		}
		x[ref.s.Id.String()] = ref.s
	}

	if newlyCreated {
		in.oa.staker.Broadcast(ref.s)
		go loopDelete(in.ctx, ref.s.OnClose(), in.reqClose.stakerCloseC, ref.s.Id, in.ws)
	}

	return nil
}

func (e1 Router) StakerByStake(stakeId sgo.PublicKey) (skr.Staker, error) {
	errorC := make(chan error, 1)
	ansC := make(chan skr.Staker, 1)
	e1.internalC <- func(in *internal) {
		ref, present := in.l_staker.byStake[stakeId.String()]
		if !present {
			errorC <- errors.New("no staker")
			return
		}
		errorC <- nil
		ansC <- ref.s
	}
	var err error
	doneC := e1.ctx.Done()
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return skr.Staker{}, err
	}
	return <-ansC, nil
}
