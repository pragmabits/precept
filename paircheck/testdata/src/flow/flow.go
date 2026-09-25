package flow

import "resource"

func linear() {
	r := resource.New()
	r.Open()
	r.Use()
	r.Close()
}

func missing() {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
}

func early(fail bool) {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	if fail {
		return
	}
	r.Close()
}

func bothBranches(ok bool) {
	r := resource.New()
	r.Open()
	if ok {
		r.Close()
	} else {
		r.Close()
	}
}

func oneBranch(ok bool) {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	if ok {
		r.Close()
	}
}

func loop(items []int) {
	r := resource.New()
	for range items {
		r.Open()
		r.Close()
	}
}

func loopLeak(items []int) {
	r := resource.New()
	for range items {
		r.Open() // want `\[resource\] Open requires Close on r before function exit`
	}
}

func switched(kind int) {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	switch kind {
	case 1:
		r.Close()
	case 2:
		return
	default:
		r.Close()
	}
}

func deferred() {
	r := resource.New()
	r.Open()
	defer r.Close()
	r.Use()
}

func conditionalDefer(ok bool) {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	if ok {
		defer r.Close()
	}
}

func multipleDefers() {
	r := resource.New()
	r.Open()
	defer r.Use()
	defer r.Close()
}

func repeated() {
	r := resource.New()
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	r.Open()
	r.Close()
}

func satisfierWithoutTrigger() {
	r := resource.New()
	r.Close()
}

func alternatives(commit bool) {
	r := resource.New()
	r.Begin()
	if commit {
		r.Commit()
		return
	}
	r.Rollback()
}

func noAlternative(commit bool) {
	r := resource.New()
	r.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on r before function exit`
	if commit {
		r.Commit()
	}
}
