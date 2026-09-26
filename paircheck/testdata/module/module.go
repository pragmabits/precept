// Package module is the root package of a module, which a rule names as "."
// and the packages below it as "./" followed by their path in the module.
package module

type Lease struct{}

func Acquire() *Lease      { return &Lease{} }
func Release(lease *Lease) {}
