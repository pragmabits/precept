// Package readme holds the example the README opens with, verbatim.
package readme

import "database/sql"

func Rename(db *sql.DB, from, to string) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE names SET name = ? WHERE name = ?`, to, from); err != nil {
		return err // tx is still open on this path
	}
	return tx.Commit()
}
