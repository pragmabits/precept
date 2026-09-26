// Package lease has an API whose value must be released even when acquiring
// it failed: the rules lease and lease-strict set open-on-error.
package lease

type Lease struct{}

func Acquire(name string) (*Lease, error) { return &Lease{}, nil }

func (l *Lease) Release() {}
