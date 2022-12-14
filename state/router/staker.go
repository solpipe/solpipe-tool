package router

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	skr "github.com/solpipe/solpipe-tool/state/staker"
	"github.com/solpipe/solpipe-tool/state/sub"
)

type refStaker struct {
	s skr.Staker
}

type lookUpStaker struct {
	byId                    map[string]*refStaker
	byStake                 map[string]*refStaker
	stakeReceiptWithNoStake map[string]map[string]sub.StakerReceiptGroup // manager id->receipt->staker receipt
	//delegationStreamC       <-chan sgows.Result
	//delegationErrorC        <-chan error
}

func (in *internal) createLookupStaker() (*lookUpStaker, error) {

	return &lookUpStaker{
		byId:                    make(map[string]*refStaker),
		byStake:                 make(map[string]*refStaker),
		stakeReceiptWithNoStake: make(map[string]map[string]sub.StakerReceiptGroup),
	}, nil
}

func (in *internal) on_stake(obj sub.StakeGroup) error {
	id := obj.Id
	if !obj.IsOpen {
		y, present := in.l_staker.byId[id.String()]
		if present {
			y.s.Close()
			delete(in.l_staker.byId, id.String())
			delete(in.l_staker.stakeReceiptWithNoStake, id.String())
		}
		return nil
	}

	data := obj.Data
	var err error

	newlyCreated := false
	ref, present := in.l_staker.byId[id.String()]
	if present {
		ref.s.Update(obj)
	} else {
		ref = new(refStaker)
		ref.s, err = skr.CreateStake(in.ctx, obj)
		if err != nil {
			return err
		}
		newlyCreated = true
		in.l_staker.byId[id.String()] = ref
		in.l_staker.byStake[data.Stake.String()] = ref
		x, present := in.l_staker.stakeReceiptWithNoStake[id.String()]
		if present {
			for _, sr := range x {
				ref.s.UpdateReceipt(sr)
				in.on_stake_receipt_extra(sr, ref)
			}
			delete(in.l_staker.stakeReceiptWithNoStake, id.String())
		}

	}

	// look up receipt
	// data.Receipt

	if newlyCreated {
		in.oa.staker.Broadcast(ref.s)
		go loopDelete(in.ctx, ref.s.OnClose(), in.reqClose.stakerCloseC, ref.s.Id, in.ws)
	}

	return nil
}

func (in *internal) on_stake_receipt(obj sub.StakerReceiptGroup) error {
	ref, present := in.l_staker.byId[obj.Data.Manager.String()]
	if present {
		ref.s.UpdateReceipt(obj)
		in.on_stake_receipt_extra(obj, ref)
	} else {
		x, present := in.l_staker.stakeReceiptWithNoStake[obj.Data.Manager.String()]
		if !present {
			x = make(map[string]sub.StakerReceiptGroup)
			in.l_staker.stakeReceiptWithNoStake[obj.Data.Manager.String()] = x
		}
		x[obj.Data.Receipt.String()] = obj
	}

	return nil
}

type stakerManagerExtra struct {
	mgr cba.StakerManager
	obj sub.StakerReceiptGroup
}

// tell the receipt that there is stake for the validator
func (in *internal) on_stake_receipt_extra(obj sub.StakerReceiptGroup, ref *refStaker) {
	data, err := ref.s.Data()
	if err != nil {
		log.Error(err)
		return
	}
	r, present := in.l_receipt.byId[obj.Data.Receipt.String()]
	if present {
		r.UpdateStaker(obj, data.Manager)
	} else {
		x, present := in.l_receipt.stakerWithNoReceipt[r.Id.String()]
		if !present {
			x = make(map[string]stakerManagerExtra)
			in.l_receipt.stakerWithNoReceipt[r.Id.String()] = x
		}
		x[ref.s.Id.String()] = stakerManagerExtra{
			mgr: data.Manager,
			obj: obj,
		}
	}
}

func (e1 Router) StakerByStake(stakeId sgo.PublicKey) (skr.Staker, error) {
	var err error
	doneC := e1.ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan skr.Staker, 1)

	select {
	case <-doneC:
		err = errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		ref, present := in.l_staker.byStake[stakeId.String()]
		if !present {
			errorC <- errors.New("no staker")
			return
		}
		errorC <- nil
		ansC <- ref.s
	}:
	}
	if err != nil {
		return skr.Staker{}, err
	}

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
