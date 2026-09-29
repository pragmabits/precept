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

// closureCommit commits in the deferred closure where the error it returns is
// nil, and rolls back where it is not.
func closureCommit(db *resource.DB) (err error) {
	tx, err := db.Begin()
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

// twoChecks returns the first of two errors it found not nil.
func twoChecks(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	first, second := work(), work()
	if first != nil {
		if second != nil {
			return first
		}
	}
	return tx.Commit()
}

// twoVariables returns the first of two errors it found not nil, both kept in
// variables a deferred closure captures.
func twoVariables(db *resource.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	other := work()
	defer func() {
		if err != nil || other != nil {
			_ = tx.Rollback()
		}
	}()
	err = work()
	if err != nil {
		if other != nil {
			return err
		}
	}
	return tx.Commit()
}

// isChecked returns the error errors.Is found to be a failure of the package.
func isChecked(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); errors.Is(err, resource.ErrBusy) {
		return err
	}
	return tx.Commit()
}

// asChecked returns the error errors.As found to hold a failure.
func asChecked(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var target *failed
	if err := work(); errors.As(err, &target) {
		return err
	}
	return tx.Commit()
}

// asTypeChecked returns the error errors.AsType found to hold a failure.
func asTypeChecked(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = work()
	if _, ok := errors.AsType[*failed](err); ok {
		return err
	}
	return tx.Commit()
}

// switched returns the error a case found equal to a failure of the package.
func switched(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	switch err := work(); err {
	case resource.ErrBusy:
		return err
	}
	return tx.Commit()
}

// isUnknownTarget compares with an error that may be nil, which proves nothing.
func isUnknownTarget(db *resource.DB, target error) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); errors.Is(err, target) {
		return err
	}
	return tx.Commit()
}

// notChecked returns where errors.Is found no failure, where the error may be
// nil.
func notChecked(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); !errors.Is(err, resource.ErrBusy) {
		return err
	}
	return tx.Commit()
}

func annotate(err error) error { return fmt.Errorf("onsuccess: %w", err) }

func annotateTwice(err error) error { return annotate(annotate(err)) }

func (f *failed) wrap() error { return f }

// maybeAnnotate returns nil for nil, so it is not always a failure.
func maybeAnnotate(err error) error {
	if err == nil {
		return nil
	}
	return annotate(err)
}

// wrappedOnce returns what a function of the package always builds as a
// failure.
func wrappedOnce(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return annotate(err)
	}
	return tx.Commit()
}

// wrappedTwice returns what a function of the package builds from another.
func wrappedTwice(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return annotateTwice(err)
	}
	return tx.Commit()
}

// wrappedByMethod returns what a method of the package always builds as a
// failure.
func wrappedByMethod(db *resource.DB, rejected bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if rejected {
		return (&failed{}).wrap()
	}
	return tx.Commit()
}

// maybeWrapped returns what a function of the package builds only sometimes.
func maybeWrapped(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return maybeAnnotate(err)
	}
	return tx.Commit()
}

var errRejected = errors.New("rejected")

var errLater error

func init() { errLater = errors.New("later") }

var errUnset error

var errCleared = errors.New("cleared")

func clearError() { errCleared = nil }

// declaredFailures returns errors of the package that hold a failure.
func declaredFailures(db *resource.DB, which int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	switch which {
	case 0:
		return errRejected
	case 1:
		return errLater
	}
	return tx.Commit()
}

// unsetFailure returns an error of the package nothing ever sets.
func unsetFailure(db *resource.DB, rejected bool) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if rejected {
		return errUnset
	}
	return tx.Commit()
}

// clearedFailure returns an error of the package that a function sets to nil.
func clearedFailure(db *resource.DB, rejected bool) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if rejected {
		return errCleared
	}
	return tx.Commit()
}

var errOpaque = resource.Failure("opaque")

var errEscaped = errors.New("escaped")

func escapedError() *error { return &errEscaped }

// opaqueFailure returns an error of the package that a function the analyzer
// does not know builds.
func opaqueFailure(db *resource.DB, rejected bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if rejected {
		return errOpaque
	}
	return tx.Commit()
}

// escapedFailure returns an error of the package whose address it hands out.
func escapedFailure(db *resource.DB, rejected bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if rejected {
		return errEscaped
	}
	return tx.Commit()
}

// closureRollsBackOnFailure decides in the deferred closure only where the
// error it returns is not nil.
func closureRollsBackOnFailure(db *resource.DB) (err error) {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	return work()
}

// closureCommitsAlways commits in the deferred closure whatever it returns.
func closureCommitsAlways(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Commit()
	}()
	return work()
}

// closureNotYetDeferred returns before it defers the closure that commits,
// with only the rollback deferred.
func closureNotYetDeferred(db *resource.DB, early bool) (err error) {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if early {
		return nil
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		}
	}()
	return work()
}
