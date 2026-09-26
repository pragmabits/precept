// Package bank uses the pgx-shaped API under the rules pool (transfer narrowed
// as the README's transaction example), pool-strict (the same with
// require-defer), conn and savepoint.
package bank

import (
	"context"

	"example.com/project/txn"
)

func work(ctx context.Context, tx txn.Tx) error {
	return tx.Exec(ctx, "SELECT 1")
}

// Leak returns on an error path with the transaction open.
func Leak(ctx context.Context, pool *txn.Pool) error {
	tx, err := pool.Begin(ctx) // want `\[pool\] Begin requires one of \[Commit, Rollback\] on tx before function exit` `\[pool-strict\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if err := work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Deferred is the idiom pgx documents: roll back in a defer, commit at the end.
func Deferred(ctx context.Context, pool *txn.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Explicit rolls back by hand on every return: a recovered panic in work would
// leave the transaction open, which only require-defer reports.
func Explicit(ctx context.Context, pool *txn.Pool) error {
	tx, err := pool.Begin(ctx) // want `\[pool-strict\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if err := work(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Helper hands the transaction to a function that commits only on success.
func Helper(ctx context.Context, pool *txn.Pool) error {
	tx, err := pool.Begin(ctx) // want `\[pool\] Begin requires one of \[Commit, Rollback\] on tx before function exit` `\[pool-strict\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	return finish(ctx, tx)
}

func finish(ctx context.Context, tx txn.Tx) error {
	if err := work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Open hands the transaction to its caller, who ends it.
func Open(ctx context.Context, pool *txn.Pool) (txn.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

type holder struct {
	tx txn.Tx
}

// Keep stores the transaction in a field and returns.
func (h *holder) Keep(ctx context.Context, pool *txn.Pool) error {
	tx, err := pool.Begin(ctx) // want `\[pool\] Begin requires one of \[Commit, Rollback\] on tx before function exit` `\[pool-strict\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	h.tx = tx
	return nil
}

// Wrapped leaves the transaction to BeginFunc: no trigger is called here.
func Wrapped(ctx context.Context, pool *txn.Pool) error {
	return txn.BeginFunc(ctx, pool, func(tx txn.Tx) error {
		return work(ctx, tx)
	})
}

// OnConn begins on a single connection and returns on an error path.
func OnConn(ctx context.Context, conn *txn.Conn) error {
	tx, err := conn.Begin(ctx) // want `\[conn\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if err := work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Savepoint begins a savepoint inside a transaction and returns on an error
// path with it open.
func Savepoint(ctx context.Context, outer txn.Tx) error {
	inner, err := outer.Begin(ctx) // want `\[savepoint\] Begin requires one of \[Commit, Rollback\] on inner before function exit`
	if err != nil {
		return err
	}
	if err := work(ctx, inner); err != nil {
		return err
	}
	return inner.Commit(ctx)
}

// SavepointReleased rolls the savepoint back in a defer.
func SavepointReleased(ctx context.Context, outer txn.Tx) error {
	inner, err := outer.Begin(ctx)
	if err != nil {
		return err
	}
	defer inner.Rollback(ctx)
	return inner.Commit(ctx)
}
