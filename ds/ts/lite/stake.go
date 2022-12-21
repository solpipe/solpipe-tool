package lite

import (
	"database/sql"
	"errors"

	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

func (sg *sqlGroup) stake() error {
	var err error
	sg.stake_select_all, err = sg.db.PrepareContext(sg.ctx, SQL_STAKE_SELECT_ALL)
	return err
}

func (e1 external) StakeAppend(list []ts.StakePoint) error {
	return e1.tx(func(tx *sql.Tx) error {
		return e1.inside_stake_append(tx, list)
	})
}

const SQL_STAKE_INSERT_1 string = `
INSERT INTO "stake" (pipeline,slot,relative_stake)
SELECT p1."id", $2, $3
FROM "pipeline" p1
WHERE p1.pipeline_id = $1;
`

func (e1 external) inside_stake_append(tx *sql.Tx, list []ts.StakePoint) error {
	insertStmt, err := tx.PrepareContext(e1.ctx, SQL_STAKE_INSERT_1)
	if err != nil {
		return err
	}
	defer insertStmt.Close()
	var r sql.Result
	var n int64
	for _, sp := range list {

		r, err = insertStmt.ExecContext(e1.ctx,
			sp.PipelineId.String(),
			sp.Slot,
			sp.Stake.Share(),
		)
		if err != nil {
			return err
		}
		n, err = r.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("failed to insert")
		}
	}
	return nil
}
