package ledger

import (
	"context"
	"errors"
	"fmt"
)

var errUnset error

func annotate(err error) error { return fmt.Errorf("ledger: annotated: %w", err) }

// Busy fails with the error errors.Is found to be the package's sentinel.
func Busy(ctx context.Context, p Provider) error {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := work(ctx); errors.Is(err, ErrRejected) {
		return err
	}
	return tx.Commit(ctx)
}

// Both fails with the first of two errors it found not nil.
func Both(ctx context.Context, p Provider) error {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	first, second := work(ctx), work(ctx)
	if first != nil {
		if second != nil {
			return first
		}
	}
	return tx.Commit(ctx)
}

// Annotated fails with what a function of the package always builds as a
// failure.
func Annotated(ctx context.Context, p Provider) error {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := work(ctx); err != nil {
		return annotate(err)
	}
	return tx.Commit(ctx)
}

// Unset returns an error of the package that nothing ever sets.
func Unset(ctx context.Context, p Provider, report Report) error {
	ctx, tx, err := p.New(ctx) // want `\[ledger\] New requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if report.Failed {
		return errUnset
	}
	return tx.Commit(ctx)
}

// CommitDeferred commits in a deferred closure where the error it returns is
// nil, and rolls back where it is not.
func CommitDeferred(ctx context.Context, p Provider) (err error) {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}
		err = tx.Commit(ctx)
	}()
	return work(ctx)
}

// RollbackDeferred rolls back in a deferred closure only where it fails, and
// never says how the transaction ends where it succeeds.
func RollbackDeferred(ctx context.Context, p Provider) (err error) {
	ctx, tx, err := p.New(ctx) // want `\[ledger\] New requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	return work(ctx)
}
