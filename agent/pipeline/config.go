package pipeline

import (
	"errors"

	"github.com/solpipe/solpipe-tool/proxy/relay"
	pipe "github.com/solpipe/solpipe-tool/state/pipeline"
	sgo "github.com/SolmateDev/solana-go"
)

type Configuration struct {
	ProgramIdCba *sgo.PublicKey
	Pipeline     *sgo.PublicKey
	Wallet       *sgo.PrivateKey
	Settings     *pipe.PipelineSettings
}

func (config *Configuration) Check() error {

	if config.Wallet == nil {
		return errors.New("no wallet private key")
	}
	if config.ProgramIdCba == nil {
		return errors.New("no program id")
	}
	if config.Pipeline == nil {
		return errors.New("no pipeline id")
	}

	if config.Settings == nil {
		return errors.New("no settings")
	}
	err := config.Settings.Check()
	if err != nil {
		return err
	}

	return nil
}

type InitializationArg struct {
	Relay   *relay.Configuration
	Program *Configuration
}

func (args *InitializationArg) Check() error {
	if args.Relay == nil {
		return errors.New("no relay configuration")
	}

	if args.Program == nil {
		return errors.New("no configuration")
	}
	err := args.Program.Check()
	if err != nil {
		return err
	}

	return nil
}

func (args *InitializationArg) Admin() sgo.PrivateKey {
	return args.Relay.Admin
}
