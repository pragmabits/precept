package exits

import (
	"log"
	"os"
	"runtime"
	"testing"

	"resource"
)

func explicitPanic(r *resource.Resource) {
	r.Open()
	panic("boom")
}

func explicitPanicDeferred(r *resource.Resource) {
	r.Open()
	defer r.Close()
	panic("boom")
}

func goexitDeferred(r *resource.Resource) {
	r.Open()
	defer r.Close()
	runtime.Goexit()
}

func failNowDeferred(t *testing.T, r *resource.Resource) {
	r.Open()
	defer r.Close()
	t.FailNow()
}

func fatalDeferred(t *testing.T, r *resource.Resource) {
	r.Open()
	defer r.Close()
	t.Fatal("stop")
}

func exitDeferred(r *resource.Resource) {
	r.Open()
	defer r.Close()
	os.Exit(1)
}

func logFatalDeferred(r *resource.Resource) {
	r.Open()
	defer r.Close()
	log.Fatal("stop")
}

func exitAfterClose(r *resource.Resource) {
	r.Open()
	r.Close()
	os.Exit(1)
}

func quit() {
	os.Exit(2)
}

func abort() {
	panic("abort")
}

func mixed(fail bool) {
	if fail {
		os.Exit(1)
	}
	panic("mixed")
}

func block() {
	for {
	}
}

func ownExit(r *resource.Resource) {
	r.Open()
	defer r.Close()
	quit()
}

func ownAbort(r *resource.Resource) {
	r.Open()
	defer r.Close()
	abort()
}

func ownMixed(r *resource.Resource) {
	r.Open()
	defer r.Close()
	mixed(true)
}

func ownBlock(r *resource.Resource) {
	r.Open()
	block()
}

func importedExit(r *resource.Resource) {
	r.Open()
	defer r.Close()
	resource.Quit()
}

func returns(r *resource.Resource) {
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
}
