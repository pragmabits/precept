// Package handle declares a type with the methods of resource.Opener, and
// does not import resource.
package handle

type Handle struct{}

func New() *Handle { return &Handle{} }

func (h *Handle) Open()  {}
func (h *Handle) Close() {}
