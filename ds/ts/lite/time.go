package lite

import (
	"database/sql"

	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

const SQL_TIME_INSERT string = `
INSERT INTO "time" (slot) VALUES ($1);
`

func (e1 external) insert_time(is ts.InitialState) error {
	ctx := e1.ctx
	tx, err := e1.db.Begin()
	if err != nil {
		return err
	}

	sqlStmt, err := tx.PrepareContext(ctx, SQL_TIME_INSERT)
	if err != nil {
		return err
	}
	defer sqlStmt.Close()
	for i := is.Start; i < is.Finish; i++ {
		_, err = sqlStmt.ExecContext(ctx, i)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

const SQL_TIME_SELECT_MAX string = `
SELECT max(t1.slot) FROM "time" t1;
`

func (sg *sqlGroup) time() error {
	var err error
	sg.slot_max, err = sg.db.PrepareContext(sg.ctx, SQL_TIME_SELECT_MAX)
	return err
}

func (e1 external) slot_max() (max uint64, err error) {
	result, err := e1.sg.slot_max.Query()
	if err != nil {
		return
	}
	defer result.Close()
	max = 0
	if !result.Next() {
		return
	}
	err = result.Scan(&max)
	if err != nil {
		return
	}
	return
}

func (e1 external) TimeAppend(length uint64) (high uint64, err error) {
	high, err = e1.slot_max()
	if err != nil {
		return
	}
	err = e1.tx(func(tx *sql.Tx) error {
		return e1.inside_time_append(tx, high+1, length)
	})
	if err != nil {
		return
	}
	high, err = e1.slot_max()
	if err != nil {
		return
	}
	return
}

func (e1 external) inside_time_append(tx *sql.Tx, start uint64, length uint64) error {
	ctx := e1.ctx
	sqlStmt, err := tx.PrepareContext(ctx, SQL_TIME_INSERT)
	if err != nil {
		return err
	}
	defer sqlStmt.Close()
	for i := uint64(0); i < length; i++ {
		_, err = sqlStmt.ExecContext(ctx, start+i)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
