package ambiguous

import "resource"

func withoutSlot(cache *resource.Cache, old *resource.Conn) {
	fresh := cache.Swap(old)
	_ = fresh
}

func stillChecked() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	_ = conn
}
