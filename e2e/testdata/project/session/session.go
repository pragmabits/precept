// Package session declares the interface of a session store, which the rule
// session is written on.
package session

type Store interface {
	Open()
	Close()
}
