package requiredefer

import (
	"sync"

	"resource"
)

func explicit(db *resource.DB, fail bool) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	if fail {
		_ = tx.Rollback()
		return resource.ErrBusy
	}
	return tx.Commit()
}

func deferred(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return tx.Commit()
}

func deferredClosure(db *resource.DB) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	return tx.Commit()
}

func handed(db *resource.DB) (*resource.Tx, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func unlockedByHand(mu *sync.Mutex) {
	mu.Lock() // want `\[lock\] Lock requires Unlock on mu before function exit`
	mu.Unlock()
}

func unlockedByDefer(mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()
}
