package closurenone

import "resource"

func closure(r *resource.Resource) {
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	defer func() {
		r.Close()
	}()
}

func direct(r *resource.Resource) {
	r.Open()
	defer r.Close()
}
