// Package dependency is a module of its own, which the module above requires
// through a replace and does not vendor: -mod=vendor fails to load it.
package dependency

func Value() int { return 0 }
