package validator

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	sgo "github.com/SolmateDev/solana-go"
	log "github.com/sirupsen/logrus"
	pba "github.com/solpipe/solpipe-tool/proto/admin"
	"github.com/solpipe/solpipe-tool/util"
)

type adminExternal struct {
	pba.UnimplementedValidatorServer
	agent Agent
}

func (e1 Agent) AttachAdmin(s *grpc.Server) error {

	a := adminExternal{agent: e1}
	pba.RegisterValidatorServer(s, a)

	return nil
}

func (a adminExternal) GetDefault(
	ctx context.Context,
	req *pba.Empty,
) (*pba.ValidatorSettings, error) {
	log.Debug("validator get - 1")
	doneC := ctx.Done()
	errorC := make(chan error, 1)
	ansC := make(chan *pba.ValidatorSettings, 1)
	var err error
	select {
	case <-doneC:
		err = errors.New("canceled")
	case a.agent.internalC <- func(in *internal) {
		if in.settings == nil {
			errorC <- errors.New("no settings")
			return
		} else {
			errorC <- nil
			ansC <- util.CopyValidatorSettings(in.settings)
		}
	}:
	}
	log.Debug("validator get - 2")
	if err != nil {
		return nil, err
	}
	log.Debug("validator get - 3")
	select {
	case <-doneC:
		err = errors.New("canceled")
	case err = <-errorC:
	}
	if err != nil {
		return nil, err
	}
	return <-ansC, nil
}

func (a adminExternal) SetDefault(
	ctx context.Context,
	req *pba.ValidatorSettings,
) (*pba.ValidatorSettings, error) {

	log.Debug("validator settings - 1")
	newReq := new(pba.ValidatorSettings)
	util.CopyMessage(req, newReq)

	if 0 < len(req.PipelineId) {
		// make sure a pipeline actually exists before interrupting loopInternal

		pipelineId, err := sgo.PublicKeyFromBase58(req.PipelineId)
		if err != nil {
			return nil, err
		}

		_, err = a.agent.router.PipelineById(pipelineId)
		if err != nil {
			return nil, err
		}
	}
	var err error
	doneC := ctx.Done()
	errorC := make(chan error)
	select {
	case <-doneC:
		err = errors.New("canceled")
	case a.agent.internalC <- func(in *internal) {
		errorC <- in.on_settings(newReq)
	}:
	}
	if err != nil {
		return nil, err
	}
	return req, nil
}
