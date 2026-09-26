// Package txn has the transaction API of pgx v5.11.0, with the signatures the
// rules depend on: a pool and a connection begin a transaction, and a
// transaction begins a savepoint, commits and rolls back. Every call takes a
// context beside the transaction.
package txn

import "context"

type Tx interface {
	Begin(ctx context.Context) (Tx, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Exec(ctx context.Context, sql string) error
}

type Pool struct{}

func (p *Pool) Begin(ctx context.Context) (Tx, error) { return nil, nil }

type Conn struct{}

func (c *Conn) Begin(ctx context.Context) (Tx, error) { return nil, nil }

// BeginFunc runs fn in a transaction it begins and ends, as pgx.BeginFunc does.
func BeginFunc(ctx context.Context, pool *Pool, fn func(Tx) error) error { return nil }
