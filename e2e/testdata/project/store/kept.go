package store

import "database/sql"

type finisher struct {
	rollback func() error
}

// RollbackKept rolls back through the method value it kept in a field, which
// the transaction rule does not take as a transfer.
func (f *finisher) RollbackKept(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	f.rollback = tx.Rollback
	defer f.rollback()
	return work(tx)
}
