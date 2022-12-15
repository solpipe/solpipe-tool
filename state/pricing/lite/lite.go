package lite

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	pr "github.com/solpipe/solpipe-tool/state/pricing"
)

type external struct {
	ctx       context.Context
	db        *sql.DB
	internalC chan<- func(*internal)
}

func Create(
	ctx context.Context,
	filePath string,
) (pr.Handle, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}
	internalC := make(chan func(*internal))
	e1 := external{
		ctx:       ctx,
		db:        db,
		internalC: internalC,
	}
	go loopInternal(ctx, internalC, db)

	return e1, nil

}
