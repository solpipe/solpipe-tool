package web

import (
	"context"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	cba "github.com/solpipe/cba"
	"github.com/solpipe/solpipe-tool/state/sub"
	val "github.com/solpipe/solpipe-tool/state/validator"
)

type ValidatorWithStake struct {
	Id   sgo.PublicKey
	Info val.StakeStatus
}

type validatorChannelGroup struct {
	dataC  chan<- sub.ValidatorGroup
	stakeC chan<- ValidatorWithStake
}

type validatorChannelGroupInternal struct {
	dataC  <-chan sub.ValidatorGroup
	stakeC <-chan ValidatorWithStake
}

func createValidatorPair() (validatorChannelGroup, validatorChannelGroupInternal) {
	dataC := make(chan sub.ValidatorGroup)
	stakeC := make(chan ValidatorWithStake)

	return validatorChannelGroup{
			dataC:  dataC,
			stakeC: stakeC,
		},
		validatorChannelGroupInternal{
			dataC:  dataC,
			stakeC: stakeC,
		}

}

func (e1 external) ws_validator(
	clientCtx context.Context,
	errorC chan<- error,
	valOut validatorChannelGroup,
) {
	log.Debug("fetching validators")
	list, err := e1.router.AllValidator()
	if err != nil {
		errorC <- err
		return
	}
	log.Debugf("fetched %d validators", len(list))
	for i := 0; i < len(list); i++ {
		d, err := list[i].Data()
		if err != nil {
			errorC <- err
			return
		}
		valOut.dataC <- sub.ValidatorGroup{
			Id:     list[i].Id,
			Data:   d,
			IsOpen: true,
		}
		go e1.ws_on_validator(errorC, clientCtx, list[i], d, valOut)
	}
}

func (e1 external) ws_on_validator(
	errorC chan<- error,
	ctx context.Context,
	v val.Validator,
	data cba.ValidatorManager,
	valOut validatorChannelGroup,
) {
	serverDoneC := e1.ctx.Done()
	doneC := ctx.Done()

	id := v.Id

	dataSub := v.OnStats()
	defer dataSub.Unsubscribe()

	stakeSub := v.OnStake()
	defer stakeSub.Unsubscribe()
	var err error

	valOut.dataC <- sub.ValidatorGroup{
		Id:     id,
		Data:   data,
		IsOpen: true,
	}

	{
		var mystake uint64
		var totalstake uint64
		mystake, totalstake, err = v.StakeRatio()
		if err != nil {
			loopValidatorFinish(id, valOut, errorC, err)
			return
		}
		valOut.stakeC <- ValidatorWithStake{
			Id: id,
			Info: val.StakeStatus{
				Activated: mystake,
				Total:     totalstake,
			},
		}
	}

out:
	for {
		select {
		case <-serverDoneC:
			break out
		case <-doneC:
			break out
		case err = <-stakeSub.ErrorC:
			break out
		case info := <-stakeSub.StreamC:
			select {
			case <-serverDoneC:
				break out
			case <-doneC:
				break out
			case valOut.stakeC <- ValidatorWithStake{
				Id:   id,
				Info: info,
			}:
			}
		case err = <-dataSub.ErrorC:
			break out
		case d := <-dataSub.StreamC:
			select {
			case <-serverDoneC:
				break out
			case <-doneC:
				break out
			case valOut.dataC <- sub.ValidatorGroup{
				Id:     id,
				Data:   d,
				IsOpen: true,
			}:
			}
		}
	}
	if err != nil {
		loopValidatorFinish(id, valOut, errorC, err)
	}
}

func loopValidatorFinish(
	id sgo.PublicKey,
	valOut validatorChannelGroup,
	errorC chan<- error,
	err error,
) {
	valOut.dataC <- sub.ValidatorGroup{
		Id:     id,
		IsOpen: false,
	}

	errorC <- err
}
