//go:build precept

// Package handle is built only under the precept build tag: a rule on it
// validates under that tag, and names nothing without it.
package handle

type Handle struct{}

func Open() *Handle { return &Handle{} }

func (h *Handle) Close() {}
