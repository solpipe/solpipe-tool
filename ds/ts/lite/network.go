package lite

import (
	"database/sql"

	ts "github.com/solpipe/solpipe-tool/ds/ts"
)

const SQL_NETWORK_INSERT_1 string = `
INSERT INTO "network" (slot,tps) VALUES ($1,$2);
`

func (e1 external) NetworkAppend(list []ts.NetworkPoint) error {
	return e1.tx(func(tx *sql.Tx) error {
		return e1.inside_network_append(tx, list)
	})
}

func (e1 external) inside_network_append(tx *sql.Tx, list []ts.NetworkPoint) error {
	insertStmt, err := tx.PrepareContext(e1.ctx, SQL_NETWORK_INSERT_1)
	if err != nil {
		return err
	}
	defer insertStmt.Close()
	for _, s := range list {
		_, err = insertStmt.Exec(s.Slot, s.Status.AverageTransactionsPerSecond)
		if err != nil {
			return err
		}
	}
	return nil
}

const SQL_STAKE_SELECT_ALL string = `
SELECT p1.pipeline_id
FROM "pipeline" p1
`
