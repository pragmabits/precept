package onsuccess

import (
	"errors"
	"fmt"

	"resource"
)

func work() error { return nil }

func wrap(err error) error { return err }

type failed struct{}

func (*failed) Error() string { return "failed" }

// noCommit returns what work returns, and decides nothing on the path.
func noCommit(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return work()
}

// verdict returns nil on a failed verdict without saying how it ends.
func verdict(db *resource.DB, rejected bool) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return err
	}
	if rejected {
		return nil
	}
	return tx.Commit()
}

// verdictExplicit rolls back in the return of the failed verdict.
func verdictExplicit(db *resource.DB, rejected bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return err
	}
	if rejected {
		return tx.Rollback()
	}
	return tx.Commit()
}

// deferred commits in the return, with the rollback deferred for the error and
// the panic.
func deferred(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return tx.Commit()
}

// committedEarly commits, then returns nil.
func committedEarly(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// failures returns each error the analyzer knows to be non-nil.
func failures(db *resource.DB, which int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	switch which {
	case 0:
		return errors.New("zero")
	case 1:
		return fmt.Errorf("one: %w", resource.ErrBusy)
	case 2:
		return resource.ErrBusy
	case 3:
		return &failed{}
	}
	if err := work(); err != nil {
		return resource.Wrap(err)
	}
	return tx.Commit()
}

// unlisted wraps with a function the configuration does not list.
func unlisted(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return wrap(err)
	}
	return tx.Commit()
}

// swallowed returns nil where work failed, which is a success.
func swallowed(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return nil
	}
	return tx.Commit()
}

// named returns the error a deferred closure reads.
func named(db *resource.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = work(); err != nil {
		return err
	}
	return tx.Commit()
}

// namedNaked sets the error, then returns without naming it.
func namedNaked(db *resource.DB, rejected bool) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if rejected {
		err = fmt.Errorf("rejected")
		return
	}
	return tx.Commit()
}

// reset clears the error it checked, and returns a success.
func reset(db *resource.DB) (err error) {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = work(); err != nil {
		err = nil
		return
	}
	return tx.Commit()
}

// closureCommit commits in the deferred closure: that is not a call on the
// path.
func closureCommit(db *resource.DB) (err error) {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return work()
}

// handed returns the transaction, and its obligation with it.
func handed(db *resource.DB) (*resource.Tx, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// uncertain checks the trigger's error in a way the analyzer does not read.
func uncertain(db *resource.DB) error {
	tx, err := db.Begin()
	if errors.Is(err, resource.ErrBusy) {
		return err
	}
	defer tx.Rollback()
	return nil
}

// noError cannot report a failure, so its every return is a success.
func noError(db *resource.DB) {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return
	}
	defer tx.Rollback()
	_ = work()
}

// noErrorDecided commits on its path.
func noErrorDecided(db *resource.DB) {
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	_ = tx.Commit()
}
