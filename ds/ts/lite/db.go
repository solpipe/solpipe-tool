package lite

import "database/sql"

func (e1 external) tx(cb func(*sql.Tx) error) error {
	tx, err := e1.db.BeginTx(e1.ctx, nil)
	if err != nil {
		return err
	}
	err = cb(tx)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
