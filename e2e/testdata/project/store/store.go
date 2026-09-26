// Package store uses database/sql under the README's transaction rule, whose
// transfer is narrowed, and under begintx, which keeps the default transfer.
package store

import (
	"context"
	"database/sql"
)

func work(tx *sql.Tx) error {
	_, err := tx.Exec("SELECT 1")
	return err
}

// Deferred rolls back in a defer and commits at the end.
func Deferred(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Open hands the transaction to its caller, who ends it.
func Open(db *sql.DB) (*sql.Tx, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// Helper hands the transaction to a function that commits only on success.
func Helper(db *sql.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	return finish(tx)
}

func finish(tx *sql.Tx) error {
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type holder struct {
	tx *sql.Tx
}

// Keep stores the transaction in a field and returns.
func (h *holder) Keep(db *sql.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	h.tx = tx
	return nil
}

// LeakTx returns on an error path with the transaction open. It calls Exec on
// the transaction itself: handing it to a helper would take the obligation
// along under the default transfer.
func LeakTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil) // want `\[begintx\] BeginTx requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if _, err := tx.Exec("SELECT 1"); err != nil {
		return err
	}
	return tx.Commit()
}

// HelperTx hands the transaction to a helper: the default transfer takes the
// obligation along.
func HelperTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	return finish(tx)
}

// KeepTx stores the transaction in a field: the default transfer takes the
// obligation along.
func (h *holder) KeepTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	h.tx = tx
	return nil
}

// PerTurn begins once per turn and commits only the last transaction: every
// earlier one is left open.
func PerTurn(db *sql.DB, n int) error {
	var tx *sql.Tx
	for range n {
		var err error
		tx, err = db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
		if err != nil {
			return err
		}
	}
	if tx == nil {
		return nil
	}
	return tx.Commit()
}

// PerTurnCommitted commits every transaction it begins.
func PerTurnCommitted(db *sql.DB, n int) error {
	for range n {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// DeferredHelper hands the transaction to a helper that runs at exit: the
// README's transaction rule, with argument narrowed, reports it.
func DeferredHelper(db *sql.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	defer finish(tx)
	return nil
}

// DeferredHelperTx does the same under the default transfer, which takes the
// obligation along with the argument.
func DeferredHelperTx(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer finish(tx)
	return nil
}
