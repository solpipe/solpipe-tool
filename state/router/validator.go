package router

import (
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	rpt "github.com/solpipe/solpipe-tool/state/receipt"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type refValidator struct {
	v    val.Validator
	data cba.ValidatorManager
}

type lookUpValidator struct {
	byId                   map[string]*refValidator
	byVote                 map[string]*refValidator
	receiptWithNoValidator map[string]map[string]rpt.Receipt // validator id->receipt id->receipt
}

func createLookupValidator() *lookUpValidator {
	return &lookUpValidator{
		byId:                   make(map[string]*refValidator),
		byVote:                 make(map[string]*refValidator),
		receiptWithNoValidator: make(map[string]map[string]rpt.Receipt),
	}
}

type ValidatorWithData struct {
	V    val.Validator
	Data cba.ValidatorManager
}

func (in *internal) lookup_add_validator(v val.Validator, data cba.ValidatorManager) *refValidator {
	ref := &refValidator{v: v, data: data}
	in.l_validator.byId[v.Id.String()] = ref
	in.l_validator.byVote[data.Vote.String()] = ref

	return ref
}

func (in *internal) on_validator(obj sub.ValidatorGroup) error {
	id := obj.Id
	if !obj.IsOpen {
		y, present := in.l_validator.byId[id.String()]
		if present {
			y.v.Close()
			delete(in.l_validator.byId, id.String())
		}
		return nil
	}
	validatorData := obj.Data
	log.Debugf("on validator (%s)", id.String())
	ref, present := in.l_validator.byId[id.String()]
	newlyCreated := false
	if !present {
		log.Debugf("creating validator (%s)", id.String())
		activeStakedSub := in.network.OnVoteStake(validatorData.Vote)
		totalStakeSub := in.network.OnTotalStake()

		v, err := val.CreateValidator(
			in.ctx,
			id,
			&validatorData,
			in.rpc,
			activeStakedSub,
			totalStakeSub,
		)
		if err != nil {
			return err
		}
		newlyCreated = true
		ref = in.lookup_add_validator(v, validatorData)
	}

	{
		x, present := in.l_validator.receiptWithNoValidator[id.String()]
		if present {
			for _, r := range x {
				ref.v.UpdateReceipt(r)
			}
			delete(in.l_validator.receiptWithNoValidator, id.String())
		}
	}

	// send validator to payout
	for _, selectedPeriod := range obj.Data.Ring {
		if !selectedPeriod.HasValidatorWithdrawn {
			receipt, present := in.l_receipt.byId[selectedPeriod.Receipt.String()]
			if present {
				receiptData, err := receipt.Data()
				if err == nil {
					payout, present := in.l_payout.byId[receiptData.Payout.String()]
					if present {
						payoutData, err := payout.p.Data()
						if err == nil {
							pipeline, present := in.l_pipeline.byId[payoutData.Pipeline.String()]
							if present {
								pipeline.UpdateValidatorByVote(
									ref.v,
									payoutData.Period.Start,
									payoutData.Period.Start+payoutData.Period.Length,
								)
							}
						}
					}
				}
			}
		}
	}

	if newlyCreated {
		in.oa.validator.Broadcast(ValidatorWithData{
			V:    ref.v,
			Data: validatorData,
		})
		go loopDelete(in.ctx, ref.v.OnClose(), in.reqClose.validatorCloseC, ref.v.Id, in.ws)
	}

	return nil
}
