package deferfirst

import (
	"context"
	"fmt"
	"sync"

	"resource"
)

func work() error { return nil }

func deferredFirst(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()
	if err := work(); err != nil {
		return err
	}
	return tx.Commit()
}

func callBeforeDefer(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx deferred before any other call`
	if err != nil {
		return err
	}
	if err := work(); err != nil {
		return err
	}
	defer tx.Rollback()
	return tx.Commit()
}

func deferredClosureFirst(db *resource.DB) (err error) {
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

func lockedAndUnlocked(mu *sync.Mutex) {
	mu.Lock()
	mu.Unlock()
}

func workWhileLocked(mu *sync.Mutex) {
	mu.Lock() // want `\[lock\] Lock requires Unlock on mu deferred before any other call`
	_ = work()
	mu.Unlock()
}

func builtinBeforeDefer(mu *sync.Mutex, items []int) int {
	mu.Lock()
	count := len(items)
	defer mu.Unlock()
	return count
}

func argumentOfTheDefer(ctx context.Context, store *resource.Store) error {
	current, err := store.Begin(ctx)
	if err != nil {
		return err
	}
	defer current.Rollback(context.WithoutCancel(ctx))
	return current.Commit(ctx)
}
