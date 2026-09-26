// Package ledger opens a transaction through a provider and rolls it back in a
// defer, under the ledger rule, which is on-success: every return that is not a
// failure has to say, with a call, how the transaction ends.
package ledger

import (
	"context"
	"errors"
	"fmt"
)

type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Provider interface {
	New(ctx context.Context) (context.Context, Transaction, error)
}

type Report struct {
	Failed bool
}

var ErrRejected = errors.New("rejected")

func work(ctx context.Context) error { return nil }

// Wrap adds context to an error: the configuration lists it as a failure.
func Wrap(err error) error { return fmt.Errorf("ledger: %w", err) }

// NoCommit never commits: its write is lost without an error.
func NoCommit(ctx context.Context, p Provider) error {
	ctx, tx, err := p.New(ctx) // want `\[ledger\] New requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return work(ctx)
}

// Store returns a failed verdict without saying how the transaction ends.
func Store(ctx context.Context, p Provider, report Report) (Report, error) {
	ctx, tx, err := p.New(ctx) // want `\[ledger\] New requires one of \[Commit, Rollback\] called on tx before a return without error`
	if err != nil {
		return report, err
	}
	defer tx.Rollback(ctx)
	if err := work(ctx); err != nil {
		return report, err
	}
	if report.Failed {
		return report, nil
	}
	return report, tx.Commit(ctx)
}

// StoreExplicit rolls back in the return of the failed verdict.
func StoreExplicit(ctx context.Context, p Provider, report Report) (Report, error) {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return report, err
	}
	defer tx.Rollback(ctx)
	if err := work(ctx); err != nil {
		return report, Wrap(err)
	}
	if report.Failed {
		return report, tx.Rollback(ctx)
	}
	return report, tx.Commit(ctx)
}

// Reject fails with a sentinel, and commits otherwise.
func Reject(ctx context.Context, p Provider, report Report) error {
	ctx, tx, err := p.New(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if report.Failed {
		return ErrRejected
	}
	return tx.Commit(ctx)
}
