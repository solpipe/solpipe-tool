package lite

func (e1 external) Initialize() error {
	var err error
	sqlStmt := `
	create table foo (id integer not null primary key, name text);
	`
	_, err = e1.db.Exec(sqlStmt)
	if err != nil {
		return err
	}
	return nil
}
