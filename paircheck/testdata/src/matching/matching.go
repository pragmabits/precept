package matching

import (
	"other"
	res "resource"
)

func alias() {
	r := res.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
}

func samePackageName() {
	r := other.New()
	r.Open()
}

type wrapper struct {
	*res.Resource
}

func promoted(w wrapper) {
	w.Open() // want `\[resource\] Open requires Close on w before function exit`
}

func promotedClosed(w wrapper) {
	w.Open()
	defer w.Close()
}

func valueReceiver() {
	var v res.Value
	v.Open() // want `\[value\] Open requires Close on v before function exit`
}

func valueReceiverClosed() {
	var v res.Value
	v.Open()
	v.Close()
}

func generic(p *res.Pool[int]) {
	p.Acquire() // want `\[pool\] Acquire requires Release on p before function exit`
}

func genericReleased(p *res.Pool[string]) {
	p.Acquire()
	p.Release()
}

func throughInterface(o res.Opener) {
	o.Open() // want `\[opener\] Open requires Close on o before function exit`
}

func concreteIsNotTheInterface() {
	f := &res.File{}
	f.Open()
}
