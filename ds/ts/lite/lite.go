package lite

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

type external struct {
	ctx       context.Context
	db        *sql.DB
	internalC chan<- func(*internal)
	sg        *sqlGroup
}

func Create(
	ctx context.Context,
	filePath string,
	willInitialize bool,
) (ts.Handle, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}
	internalC := make(chan func(*internal))

	var sg *sqlGroup
	if !willInitialize {
		sg, err = sql_init(ctx, db)
		if err != nil {
			return nil, err
		}
	}

	e1 := &external{
		ctx:       ctx,
		db:        db,
		internalC: internalC,
		sg:        sg,
	}
	go loopInternal(ctx, internalC, db)

	return e1, nil

}
