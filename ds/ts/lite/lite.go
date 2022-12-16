package lite

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

type external struct {
	ctx       context.Context
	cancel    context.CancelFunc
	db        *sql.DB
	internalC chan<- func(*internal)
	sg        *sqlGroup
}

// create an in-memory sqlite instance
func Create(ctx context.Context) (main ts.Handle, err error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return
	}

	ctxC, cancel := context.WithCancel(ctx)
	main, err = create_with_init(ctxC, cancel, db)
	if err != nil {
		return
	}
	return
}

func create_with_init(
	ctx context.Context,
	cancel context.CancelFunc,
	db *sql.DB,
) (*external, error) {

	internalC := make(chan func(*internal))

	e1 := &external{
		ctx:       ctx,
		db:        db,
		internalC: internalC,
	}
	err := e1.Initialize(ts.InitialState{Start: 1, Finish: 2})
	if err != nil {
		return nil, err
	}
	e1.sg, err = sql_init(ctx, db)
	if err != nil {
		return nil, err
	}
	go loopInternal(ctx, internalC, db)

	return e1, nil

}

func (e1 external) Db() *sql.DB {
	return e1.db
}

func (e1 external) Type() ts.DbType {
	return ts.DB_TYPE_LITE
}

func (e1 external) Close() error {
	signalC := e1.CloseSignal()
	e1.cancel()
	return <-signalC
}
