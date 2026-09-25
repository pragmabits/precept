package closureevery

import "resource"

func unconditional(r *resource.Resource) {
	r.Open()
	defer func() {
		r.Close()
	}()
}

func conditional(r *resource.Resource, failed bool) {
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	defer func() {
		if failed {
			r.Close()
		}
	}()
}

func everyBranch(r *resource.Resource, failed bool) {
	r.Open()
	defer func() {
		if failed {
			r.Close()
			return
		}
		r.Close()
	}()
}
