// Package generics uses generic under handle (generic functions), box (a method
// of a generic type named without its type parameters), cache (named with
// them) and hold (a type parameter, with the slots named).
package generics

import "example.com/project/generic"

func HandleLeak() {
	handle := generic.Open(1) // want `\[handle\] Open requires Close on handle before function exit`
	_ = handle
}

func HandleClosed() {
	handle := generic.Open("text")
	defer generic.Close(handle)
}

func HandleInGeneric[T any](value T) {
	handle := generic.Open(value) // want `\[handle\] Open requires Close on handle before function exit`
	_ = handle
}

func HandleClosedInGeneric[T any](value T) {
	handle := generic.Open(value)
	defer generic.Close(handle)
}

func BoxLeak() {
	box := generic.NewBox(1.5) // want `\[box\] NewBox requires Close on box before function exit`
	_ = box
}

func BoxClosed() {
	box := generic.NewBox(1)
	defer box.Close()
}

func CacheLeak() {
	cache := generic.NewCache[string, int]() // want `\[cache\] NewCache requires Close on cache before function exit`
	_ = cache
}

func CacheClosed() {
	cache := generic.NewCache[string, int]()
	defer cache.Close()
}

func HoldLeak() {
	lease := &generic.Lease{}
	generic.Hold(lease) // want `\[hold\] Hold requires Free on lease before function exit`
}

func HoldFreed() {
	lease := &generic.Lease{}
	generic.Hold(lease)
	defer generic.Free(lease)
}

func HoldInGeneric[L generic.Leased](lease L) {
	generic.Hold(lease) // want `\[hold\] Hold requires Free on lease before function exit`
}

func FreedInGeneric[L generic.Leased](lease L) {
	generic.Hold(lease)
	defer generic.Free(lease)
}

func HoldAllLeak() {
	leases := []*generic.Lease{{}}
	generic.HoldAll(leases) // want `\[all\] HoldAll requires FreeAll on leases before function exit`
}

func HoldAllFreed() {
	leases := []*generic.Lease{{}}
	generic.HoldAll(leases)
	defer generic.FreeAll(leases)
}

func LockLeak(entries map[string]int) {
	generic.Lock(entries) // want `\[entries\] Lock requires Unlock on entries before function exit`
}

func LockUnlocked(entries map[string]int) {
	generic.Lock(entries)
	defer generic.Unlock(entries)
}
