package pipeline

import (
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	ll "github.com/solpipe/solpipe-tool/ds/list"
	dssub "github.com/solpipe/solpipe-tool/ds/sub"
	"github.com/solpipe/solpipe-tool/util"
)

type refundInfo struct {
	list     *ll.Generic[*cba.Claim]
	claimMap map[string]*ll.Node[*cba.Claim]
}

func (in *internal) init_refund() error {
	ri := new(refundInfo)
	in.refundInfo = ri

	ri.claimMap = make(map[string]*ll.Node[*cba.Claim])
	ri.list = ll.CreateGeneric[*cba.Claim]()

	return nil
}

func (in *internal) on_refund(d cba.Refunds) {
	log.Debugf("refunds=%+v", &d)
	ri := in.refundInfo
	oldList := ri.list
	newList := ll.CreateGeneric[*cba.Claim]()

	for _, claim := range d.Refunds {
		if 0 < claim.Balance {
			var newNode *ll.Node[*cba.Claim]
			c := &claim
			oldNode, present := ri.claimMap[claim.User.String()]
			if present {
				oldList.Remove(oldNode)
			}
			newNode = newList.Append(c)
			ri.claimMap[claim.User.String()] = newNode
			in.claimHome.Broadcast(*c)
		}
	}
	// tell the left over users that they have been refunded
	oldList.Iterate(func(c *cba.Claim, index uint32, deleteNode func()) error {
		delete(ri.claimMap, c.User.String())
		c.Balance = 0
		in.claimHome.Broadcast(*c)
		return nil
	})
	ri.list = newList
}

// set user=util.Zero() to get all claim updates
func (e1 Pipeline) OnClaim(user sgo.PublicKey) dssub.Subscription[cba.Claim] {
	return dssub.SubscriptionRequest(e1.updateClaimC, func(c cba.Claim) bool {
		zero := util.Zero()
		if user.Equals(zero) {
			return true
		} else if user.Equals(c.User) {
			return true
		} else {
			return false
		}
	})
}

func (e1 Pipeline) AllClaim() ([]cba.Claim, error) {
	doneC := e1.ctx.Done()
	ansC := make(chan []cba.Claim)
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		list := make([]cba.Claim, in.refundInfo.list.Size)
		in.refundInfo.list.Iterate(func(obj *cba.Claim, index uint32, deleteNode func()) error {
			list[index] = *obj
			return nil
		})
		ansC <- list
	}:
	}
	var l []cba.Claim
	select {
	case <-doneC:
		return nil, errors.New("canceled")
	case l = <-ansC:
		return l, nil
	}
}
