package lite

import (
	"database/sql"

	sgo "github.com/SolmateDev/solana-go"
)

func (e1 external) PipelineAdd(idList []sgo.PublicKey) error {
	return e1.tx(func(tx *sql.Tx) error {
		return e1.inside_pipeline_add(tx, idList)
	})
}

const SQL_PIPELINE_INSERT string = `
INSERT INTO pipeline (pipeline_id) VALUES (?);
`

func (e1 external) inside_pipeline_add(tx *sql.Tx, idList []sgo.PublicKey) error {
	insertStmt, err := tx.PrepareContext(e1.ctx, SQL_PIPELINE_INSERT)
	if err != nil {
		return err
	}
	for _, id := range idList {
		_, err = insertStmt.Exec(id.String())
		if err != nil {
			return err
		}
	}
	return nil
}

const SQL_PIPELINE_SELECT_ALL string = `
SELECT p1.pipeline_id
FROM "pipeline" p1
`

func (sg *sqlGroup) pipeline() error {
	var err error
	sg.pipeline_select, err = sg.db.PrepareContext(sg.ctx, SQL_PIPELINE_SELECT_ALL)
	return err
}

func (e1 external) PipelineList() (list []sgo.PublicKey, err error) {
	results, err := e1.sg.pipeline_select.Query()
	if err != nil {
		return
	}
	list = make([]sgo.PublicKey, 0)
	var pubkey sgo.PublicKey
	var id string

	for results.Next() {
		err = results.Scan(&id)
		if err != nil {
			return
		}
		pubkey, err = sgo.PublicKeyFromBase58(id)
		if err != nil {
			return
		}
		list = append(list, pubkey)
	}
	return
}
