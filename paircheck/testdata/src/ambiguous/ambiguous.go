package ambiguous

import "resource"

func withoutSlot(cache *resource.Cache, old *resource.Conn) {
	fresh := cache.Swap(old)
	_ = fresh
}
