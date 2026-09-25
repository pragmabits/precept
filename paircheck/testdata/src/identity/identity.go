package identity

import "resource"

func distinctTriggerResults() {
	first := resource.Dial()
	second := resource.Dial() // want `\[dial\] Dial requires Close on second before function exit`
	first.Close()
	_ = second
}

func distinctAllocations() {
	first, second := &resource.Resource{}, &resource.Resource{}
	first.Open() // want `\[resource\] Open requires Close on first before function exit`
	second.Close()
}

func parametersMayBeTheSame(first, second *resource.Resource) {
	first.Open()
	second.Close()
}

type holder struct {
	res *resource.Resource
}

func field(h *holder) {
	h.res.Open()
	defer h.res.Close()
}

func fieldOpenedTwice(h *holder) {
	h.res.Open() // want `\[resource\] Open requires Close on h.res before function exit`
	h.res.Open()
	h.res.Close()
}

type pair struct {
	first  *resource.Resource
	second *resource.Resource
}

func fieldsMayHoldTheSame(p *pair) {
	p.first.Open()
	p.second.Close()
}

func alias() {
	first := &resource.Resource{}
	first.Open()
	second := first
	second.Close()
}

func mergedValues(chooseSecond bool) {
	first, second := &resource.Resource{}, &resource.Resource{}
	chosen := first
	if chooseSecond {
		chosen = second
	}
	first.Open()
	chosen.Close()
}

func throughAny() {
	first := &resource.Resource{}
	first.Open()
	var held any = first
	held.(*resource.Resource).Close()
}

func throughAnyAnotherValue() {
	first, second := &resource.Resource{}, &resource.Resource{}
	var held any = second
	first.Open() // want `\[resource\] Open requires Close on first before function exit`
	held.(*resource.Resource).Close()
}
