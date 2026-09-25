package invisible

import "resource"

func triggerAlone() {
	conn := resource.Dial() // want `\[finish\] Dial requires Finish on conn before function exit`
	_ = conn
}

func ambiguousWithoutTheSatisfier(cache *resource.Cache, old *resource.Conn) {
	fresh := cache.Swap(old)
	_ = fresh
}
