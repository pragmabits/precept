// Package locks uses sync.Mutex under the README's lock rule, and
// sync.RWMutex's RLock under read and read-strict (read with require-defer).
package locks

import (
	"errors"
	"sync"
)

var errFailed = errors.New("failed")

type counter struct {
	mu sync.Mutex
	n  int
}

// Add unlocks the field in a defer.
func (c *counter) Add() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

// Leak returns on one path with the field locked.
func (c *counter) Leak(fail bool) error {
	c.mu.Lock() // want `\[lock\] Lock requires Unlock on c.mu before function exit`
	if fail {
		return errFailed
	}
	c.mu.Unlock()
	return nil
}

// LockInLoop locks once per turn and unlocks once: two triggers on the same
// value need two satisfiers.
func LockInLoop(mu *sync.Mutex, items []int) {
	for range items {
		mu.Lock() // want `\[lock\] Lock requires Unlock on mu before function exit`
	}
	mu.Unlock()
}

// LockPerTurn unlocks what each turn locked.
func LockPerTurn(mu *sync.Mutex, items []int) {
	for range items {
		mu.Lock()
		mu.Unlock()
	}
}

// ReadTwice read-locks twice and unlocks once.
func ReadTwice(mu *sync.RWMutex) {
	mu.RLock() // want `\[read\] RLock requires RUnlock on mu before function exit` `\[read-strict\] RLock requires RUnlock on mu before function exit`
	mu.RLock() // want `\[read-strict\] RLock requires RUnlock on mu before function exit`
	mu.RUnlock()
}

// ReadEach read-locks in a loop and defers each unlock.
func ReadEach(mu *sync.RWMutex, items []int) {
	for range items {
		mu.RLock()
		defer mu.RUnlock()
	}
}

// ReadEachExplicit unlocks each turn by hand.
func ReadEachExplicit(mu *sync.RWMutex, items []int) {
	for range items {
		mu.RLock() // want `\[read-strict\] RLock requires RUnlock on mu before function exit`
		mu.RUnlock()
	}
}

type table struct {
	mu   sync.RWMutex
	rows map[string]int
}

func (t *table) log(key string) {}

// WriteDeferredFirst defers the unlock before anything else.
func (t *table) WriteDeferredFirst(key string, value int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.log(key)
	t.rows[key] = value
}

// WriteLogsFirst logs before deferring the unlock: a panic in log would leave
// the table locked.
func (t *table) WriteLogsFirst(key string, value int) {
	t.mu.Lock() // want `\[write-first\] Lock requires Unlock on t.mu deferred before any other call`
	t.log(key)
	defer t.mu.Unlock()
	t.rows[key] = value
}
