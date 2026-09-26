// Package leases uses the lease API under lease (open-on-error) and
// lease-strict (open-on-error with require-defer).
package leases

import "example.com/project/lease"

// LeakOnError returns on the error path without releasing.
func LeakOnError(name string) error {
	held, err := lease.Acquire(name) // want `\[lease\] Acquire requires Release on held before function exit` `\[lease-strict\] Acquire requires Release on held before function exit`
	if err != nil {
		return err
	}
	held.Release()
	return nil
}

// ReleasedByHand releases on both paths without a defer.
func ReleasedByHand(name string) error {
	held, err := lease.Acquire(name) // want `\[lease-strict\] Acquire requires Release on held before function exit`
	if err != nil {
		held.Release()
		return err
	}
	held.Release()
	return nil
}

// ReleasedByDefer releases in a defer, before the error is checked.
func ReleasedByDefer(name string) error {
	held, err := lease.Acquire(name)
	defer held.Release()
	if err != nil {
		return err
	}
	return nil
}
