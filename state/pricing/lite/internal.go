package lite

import (
	"context"
	"database/sql"
	"errors"
)

type internal struct {
	ctx              context.Context
	closeSignalCList []chan<- error
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	db *sql.DB,
) {
	doneC := ctx.Done()
	defer db.Close()
	in := new(internal)
	in.ctx = ctx

out:
	for {
		select {
		case <-doneC:
			break out
		case req := <-internalC:
			req(in)
		}
	}

	in.finish(db.Close())

}

func (in *internal) finish(err error) {
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (e1 external) CloseSignal() <-chan error {
	doneC := e1.ctx.Done()
	signalC := make(chan error, 1)
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
	case e1.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	return signalC
}
