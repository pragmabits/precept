package use

import (
	"example.com/module"
	"example.com/module/resource"
)

func leaked(r *resource.Resource) {
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
}

func closed(r *resource.Resource) {
	r.Open()
	defer r.Close()
}

func leakedLease() {
	lease := module.Acquire() // want `\[lease\] Acquire requires Release on lease before function exit`
	_ = lease
}

func released() {
	lease := module.Acquire()
	defer module.Release(lease)
}
