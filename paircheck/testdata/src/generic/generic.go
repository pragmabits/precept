package generic

import "resource"

func held() {
	lease := &resource.Lease{}
	resource.Hold(lease) // want `\[hold\] Hold requires Free on lease before function exit`
}

func freed() {
	lease := &resource.Lease{}
	resource.Hold(lease)
	defer resource.Free(lease)
}

func heldInGeneric[L resource.Leased](lease L) {
	resource.Hold(lease) // want `\[hold\] Hold requires Free on lease before function exit`
}

func freedInGeneric[L resource.Leased](lease L) {
	resource.Hold(lease)
	defer resource.Free(lease)
}

func heldAll() {
	leases := []*resource.Lease{{}}
	resource.HoldAll(leases) // want `\[all\] HoldAll requires FreeAll on leases before function exit`
}

func freedAll() {
	leases := []*resource.Lease{{}}
	resource.HoldAll(leases)
	defer resource.FreeAll(leases)
}

func locked(entries map[string]int) {
	resource.Lock(entries) // want `\[entries\] Lock requires Unlock on entries before function exit`
}

func unlocked(entries map[string]int) {
	resource.Lock(entries)
	defer resource.Unlock(entries)
}
