package paircheck

import (
	"fmt"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)

// summary is what a function does with the value of a protocol that it
// receives in a parameter, by the protocol's key: discharge it on every path
// it returns by, or keep it, neither discharging it nor letting it leave on
// any. A parameter the function does anything else with has no entry, and a
// call hands the value over to it. Parameters are counted from the receiver.
// The lists are sorted, so that a fact is encoded alike every time.
type summary struct {
	Discharged []parameters
	Kept       []parameters
}

// parameters are the indices of the parameters a summary says one thing of,
// for the protocol of key.
type parameters struct {
	Key     string
	Indices []int
}

func (*summary) AFact() {}

// discharged lists the parameters s discharges the value of key in.
func (s *summary) discharged(key string) []int {
	return indicesOf(s.Discharged, key)
}

// kept lists the parameters s keeps the value of key in.
func (s *summary) kept(key string) []int {
	return indicesOf(s.Kept, key)
}

func (s *summary) String() string {
	return fmt.Sprintf("discharged %v, kept %v", s.Discharged, s.Kept)
}

func indicesOf(lists []parameters, key string) []int {
	index, found := slices.BinarySearchFunc(lists, key, compareKey)
	if !found {
		return nil
	}
	return lists[index].Indices
}

// with is lists with index added for key, sorted by key and then by index.
func with(lists []parameters, key string, index int) []parameters {
	position, found := slices.BinarySearchFunc(lists, key, compareKey)
	if !found {
		lists = slices.Insert(lists, position, parameters{Key: key})
	}
	indices := lists[position].Indices
	at, present := slices.BinarySearch(indices, index)
	if !present {
		lists[position].Indices = slices.Insert(indices, at, index)
	}
	return lists
}

func compareKey(list parameters, key string) int {
	return strings.Compare(list.Key, key)
}

// summaries is what a pass knows of the functions its searches call: those of
// the analyzed package, learned before any search, and those of the packages
// it imports, from their facts.
type summaries struct {
	pass  *analysis.Pass
	local map[*types.Func]*summary
}

// learnSummaries learns what each function of the analyzed package does with
// the value of each binding in its parameters, again until nothing is left,
// since a function can hand the value to another. It exports as a fact the
// summary of each function another package can call: an exported one.
func learnSummaries(
	pass *analysis.Pass,
	bindings []binding,
	functions []*ssa.Function,
) *summaries {
	known := &summaries{pass: pass, local: make(map[*types.Func]*summary)}
	for index := range bindings {
		bindings[index].summaries = known
	}
	for learned := true; learned; {
		learned = false
		for _, function := range functions {
			for _, current := range bindings {
				learned = known.learn(current, function) || learned
			}
		}
	}
	for function, found := range known.local {
		if function.Exported() {
			pass.ExportObjectFact(function, found)
		}
	}
	return known
}

// learn records what function does with each parameter that may hold the
// value of current, and reports whether it recorded anything new. The trigger
// and the satisfiers are known by name, and need none. A method that keeps its
// receiver says nothing: a call never hands its receiver over.
func (s *summaries) learn(current binding, function *ssa.Function) bool {
	object, ok := function.Object().(*types.Func)
	if !ok || function.Parent() != nil || len(function.Blocks) == 0 ||
		current.protocol.named(object) {
		return false
	}
	learned := false
	for index, parameter := range function.Params {
		learned = s.learnParameter(current, object, index, parameter) || learned
	}
	return learned
}

// learnParameter records what the function object declares does with the
// parameter at index, and reports whether that is new.
func (s *summaries) learnParameter(
	current binding,
	object *types.Func,
	index int,
	parameter *ssa.Parameter,
) bool {
	key := current.protocol.key()
	found := s.local[object]
	if !sameType(parameter.Type(), current.valueType) || found.holds(key, index) {
		return false
	}
	discharged, kept := current.follow(parameter.Parent(), parameter)
	receiver := index == 0 && object.Signature().Recv() != nil
	if !discharged && (!kept || receiver) {
		return false
	}
	if found == nil {
		found = new(summary)
		s.local[object] = found
	}
	if discharged {
		found.Discharged = with(found.Discharged, key, index)
		return true
	}
	found.Kept = with(found.Kept, key, index)
	return true
}

// of is the summary of function, if the pass knows one.
func (s *summaries) of(function *types.Func) (*summary, bool) {
	if s == nil {
		return nil, false
	}
	function = function.Origin()
	if found, ok := s.local[function]; ok {
		return found, true
	}
	if s.pass == nil || function.Pkg() == s.pass.Pkg {
		return nil, false
	}
	imported := new(summary)
	return imported, s.pass.ImportObjectFact(function, imported)
}

// holds reports whether s says anything of the parameter at index for key.
func (s *summary) holds(key string, index int) bool {
	return s != nil &&
		(slices.Contains(s.discharged(key), index) || slices.Contains(s.kept(key), index))
}

// follow searches function with an obligation opened on parameter at its
// start, as a helper that receives the value: whether it discharges the value
// on every path it returns by, or keeps it on every one. The keys that decide
// how the caller must discharge the value play no part in what the helper does
// with it.
func (b binding) follow(function *ssa.Function, parameter *ssa.Parameter) (discharged, kept bool) {
	helper := b
	helper.protocol.requireDefer = false
	helper.protocol.deferFirst = false
	helper.protocol.onSuccess = false
	ended := make(endings)
	(search{
		binding: helper,
		opened: obligation{
			start: function.Blocks[0],
			index: -1,
			value: parameter,
			homes: homesOf(parameter),
		},
		ended: ended,
	}).explore(false)
	if ended[endTransferred] {
		return false, false
	}
	switch {
	case ended[endDischarged] && !ended[endOpen]:
		return true, false
	case ended[endOpen] && !ended[endDischarged]:
		return false, true
	}
	return false, false
}

// summarized is what a call does with the value through a summary of its
// callee: discharge it, keep it in the arguments it names, or nothing known.
func (s search) summarized(common *ssa.CallCommon) (discharged bool, kept []int) {
	if common.IsInvoke() {
		return false, nil
	}
	callee := calleeOf(common)
	if callee == nil {
		return false, nil
	}
	found, ok := s.binding.summaries.of(callee)
	if !ok {
		return false, nil
	}
	key := s.binding.protocol.key()
	for _, index := range found.discharged(key) {
		if index < len(common.Args) && s.carries(common.Args[index]) {
			return true, nil
		}
	}
	return false, found.kept(key)
}
