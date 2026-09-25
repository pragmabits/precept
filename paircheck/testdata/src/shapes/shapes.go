package shapes

import "resource"

func resultToReceiver(db *resource.DB) {
	tx, _ := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	_ = tx
}

func resultToReceiverClosed(db *resource.DB) {
	tx, _ := db.Begin()
	defer tx.Rollback()
}

func argumentToArgument(r *resource.Resource) {
	resource.Acquire(r) // want `\[acquire\] Acquire requires Release on r before function exit`
}

func argumentToArgumentReleased(r *resource.Resource) {
	resource.Acquire(r)
	defer resource.Release(r)
}

func resultToArgument() {
	h, _ := resource.OpenHandle("path") // want `\[handle\] OpenHandle requires CloseHandle on h before function exit`
	_ = h
}

func resultToArgumentClosed() {
	h, _ := resource.OpenHandle("path")
	resource.CloseHandle(h)
}

func singleResult() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	_ = conn
}

func singleResultClosed() {
	conn := resource.Dial()
	defer conn.Close()
}

func explicitSlot(cache *resource.Cache, old *resource.Conn) {
	fresh := cache.Swap(old) // want `\[swap\] Swap requires Put on fresh before function exit`
	_ = fresh
}

func explicitSlotPut(cache *resource.Cache, old *resource.Conn) {
	fresh := cache.Swap(old)
	cache.Put(fresh)
}
