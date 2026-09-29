package paircheck

import (
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// returnsFailure reports whether ret hands back an error the path knows is not
// nil. A function whose last result is not an error has none to hand back, so
// its every return is a success.
func (s search) returnsFailure(current state, ret *ssa.Return) bool {
	results := ret.Parent().Signature.Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return false
	}
	return s.fails(current, ret.Results[last])
}

// registered is current after instruction, which may defer a closure that
// calls a satisfier.
func (s search) registered(current state, instruction ssa.Instruction) state {
	deferred, ok := instruction.(*ssa.Defer)
	if !ok {
		return current
	}
	if index, tracked := s.closures[deferred]; tracked {
		current.closures = current.closures.with(index)
	}
	return current
}

// closureDecides reports whether a closure the path deferred decides how the
// obligation ends where ret returns no failure: it calls a satisfier on every
// path where the error ret hands back is not a failure.
func (s search) closureDecides(current state, ret *ssa.Return) bool {
	result := resultVariable(ret)
	for deferred, index := range s.closures {
		if !current.closures.has(index) {
			continue
		}
		if deferredClosure, ok := closureOf(s, deferred); ok && deferredClosure.decides(result) {
			return true
		}
	}
	return false
}

// resultVariable is the variable ret reads the error it hands back from, which
// a deferred closure may read and write, or nil.
func resultVariable(ret *ssa.Return) ssa.Value {
	results := ret.Parent().Signature.Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return nil
	}
	variable, _ := variableRead(ret.Results[last])
	return variable
}

// deferredClosures indexes the closures function defers that call a
// satisfier of current on some path.
func deferredClosures(current binding, function *ssa.Function) map[*ssa.Defer]int {
	indexed := make(map[*ssa.Defer]int)
	probe := search{binding: current}
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			deferred, ok := instruction.(*ssa.Defer)
			if !ok {
				continue
			}
			if found, ok := closureOf(probe, deferred); ok && found.coversAnyPath() {
				indexed[deferred] = len(indexed)
			}
		}
	}
	return indexed
}

// returnedErrors indexes the errors function can return: the values that reach
// the error a return hands back, through phis and variables, and those
// variables. A path needs to know only these as failures.
func returnedErrors(function *ssa.Function) map[ssa.Value]int {
	indexed := make(map[ssa.Value]int)
	results := function.Signature.Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return indexed
	}
	var pending []ssa.Value
	add := func(value ssa.Value) {
		if _, known := indexed[value]; !known {
			indexed[value] = len(indexed)
			pending = append(pending, value)
		}
	}
	for _, block := range function.Blocks {
		if ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return); ok {
			add(ret.Results[last])
		}
	}
	for len(pending) > 0 {
		value := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, reached := range reaching(value) {
			add(reached)
		}
	}
	return indexed
}

// reaching lists what reaches value: the edges of a phi, the variable a load
// reads, and what is stored into a variable.
func reaching(value ssa.Value) []ssa.Value {
	var reached []ssa.Value
	if phi, ok := value.(*ssa.Phi); ok {
		reached = append(reached, phi.Edges...)
	}
	if variable, ok := variableRead(value); ok {
		reached = append(reached, variable)
	}
	if referrers := value.Referrers(); referrers != nil {
		for _, referrer := range *referrers {
			if store, ok := referrer.(*ssa.Store); ok && store.Addr == value {
				reached = append(reached, store.Val)
			}
		}
	}
	return reached
}

// remembered is current after instruction, which may store an error into a
// variable: a failure makes the variable hold one, and anything else stored
// where a failure was makes it no longer hold one.
func (s search) remembered(current state, instruction ssa.Instruction) state {
	store, ok := instruction.(*ssa.Store)
	if !ok || !s.binding.protocol.onSuccess || !types.Identical(store.Val.Type(), errorType) {
		return current
	}
	index, tracked := s.returned[store.Addr]
	if !tracked {
		return current
	}
	if s.fails(current, store.Val) {
		current.failed = current.failed.with(index)
		return current
	}
	current.failed = current.failed.without(index)
	return current
}

// fails reports whether value is an error the path knows is not nil: one built
// that way, a value a check on the path found not nil, or a load of a variable
// a failure was stored into.
func (s search) fails(current state, value ssa.Value) bool {
	if s.known(current, value) {
		return true
	}
	if variable, ok := variableRead(value); ok && s.known(current, variable) {
		return true
	}
	return s.built(value)
}

// known reports whether the path holds value, an error or a variable, as a
// failure.
func (s search) known(current state, value ssa.Value) bool {
	index, tracked := s.returned[value]
	return tracked && current.failed.has(index)
}

func (s search) built(value ssa.Value) bool {
	return s.binding.failures.built(value, make(map[*ssa.Phi]bool))
}

// along is current on the successor at position of branch. Where branch finds
// an error not nil, the path holds it as a failure, or the variable it was
// read from; where it finds it nil, the path no longer does.
func (s search) along(current state, branch *ssa.If, position int) state {
	found, ok := s.proofOf(branch.Cond)
	if !ok {
		return current
	}
	checked := found.checked
	if variable, read := variableRead(checked); read {
		checked = variable
	}
	index, tracked := s.returned[checked]
	switch {
	case !tracked:
		return current
	case position == found.failing:
		current.failed = current.failed.with(index)
	case found.nilOtherwise:
		current.failed = current.failed.without(index)
	}
	return current
}

// proof is what a branch condition proves of an error: on the successor at
// failing, that it is not nil; on the other, that it is nil when the condition
// compares it with nil, and nothing otherwise.
type proof struct {
	checked      ssa.Value
	failing      int
	nilOtherwise bool
}

// proofOf is what condition proves: a comparison with nil, a comparison
// with a failure, which is also what a case of a switch on the error does, or
// a predicate of the errors package.
func (s search) proofOf(condition ssa.Value) (proof, bool) {
	if checked, failing, ok := nilCheck(condition); ok {
		return proof{checked: checked, failing: failing, nilOtherwise: true}, true
	}
	if comparison, ok := condition.(*ssa.BinOp); ok {
		return s.equality(comparison)
	}
	return s.predicate(condition)
}

// equality is the proof of an error compared with a failure: where the two are
// equal, the error is one too.
func (s search) equality(comparison *ssa.BinOp) (proof, bool) {
	if comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return proof{}, false
	}
	checked, failure := comparison.X, comparison.Y
	if !s.built(failure) {
		checked, failure = failure, checked
	}
	if !types.Identical(checked.Type(), errorType) ||
		!s.built(failure) {
		return proof{}, false
	}
	if comparison.Op == token.NEQ {
		return proof{checked: checked, failing: 1}, true
	}
	return proof{checked: checked}, true
}

// predicate is the proof of errors.Is against a failure, of errors.As, or of
// errors.AsType through its second result: each is true only for an error that
// is not nil.
func (s search) predicate(condition ssa.Value) (proof, bool) {
	call, result, ok := predicateCall(condition)
	if !ok {
		return proof{}, false
	}
	common := call.Common()
	if !s.proves(calleeOf(common).Origin().FullName(), result, common.Args) {
		return proof{}, false
	}
	return proof{checked: common.Args[0]}, true
}

// predicateCall is the call condition is, or whose result it extracts, with
// that result, or -1 for the call itself.
func predicateCall(condition ssa.Value) (*ssa.Call, int, bool) {
	call, result := condition, -1
	if extract, ok := condition.(*ssa.Extract); ok {
		call, result = extract.Tuple, extract.Index
	}
	called, ok := call.(*ssa.Call)
	if !ok {
		return nil, 0, false
	}
	common := called.Common()
	if calleeOf(common) == nil || len(common.Args) == 0 {
		return nil, 0, false
	}
	return called, result, true
}

// proves reports whether the predicate named name, true through result, finds
// the first of arguments not nil.
func (s search) proves(name string, result int, arguments []ssa.Value) bool {
	switch name {
	case "errors.Is":
		return result < 0 && len(arguments) > 1 && s.built(arguments[1])
	case "errors.As":
		return result < 0
	case "errors.AsType":
		return result == 1
	}
	return false
}

// nilCheck is the error condition compares with nil, and the position of the
// successor taken where that error is not nil.
func nilCheck(condition ssa.Value) (ssa.Value, int, bool) {
	comparison, ok := condition.(*ssa.BinOp)
	if !ok || comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return nil, 0, false
	}
	checked := comparison.X
	switch {
	case isNil(comparison.X):
		checked = comparison.Y
	case !isNil(comparison.Y):
		return nil, 0, false
	}
	if !types.Identical(checked.Type(), errorType) {
		return nil, 0, false
	}
	if comparison.Op == token.NEQ {
		return checked, 0, true
	}
	return checked, 1, true
}

// variableRead is the variable value is loaded from, if it is a load.
func variableRead(value ssa.Value) (ssa.Value, bool) {
	load, ok := value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return nil, false
	}
	return load.X, true
}

// proven is a set of the errors a search tracks, one bit for each by its
// index. It is a string so that a state holding it stays comparable, and it
// never ends in a zero byte, so that one set has one spelling.
type proven string

func (p proven) has(index int) bool {
	return index/8 < len(p) && p[index/8]&(1<<(index%8)) != 0
}

func (p proven) with(index int) proven {
	bits := []byte(p)
	for len(bits) <= index/8 {
		bits = append(bits, 0)
	}
	bits[index/8] |= 1 << (index % 8)
	return proven(bits)
}

// includes reports whether p holds every error other holds.
func (p proven) includes(other proven) bool {
	if len(other) > len(p) {
		return false
	}
	for index := range len(other) {
		if other[index]&^p[index] != 0 {
			return false
		}
	}
	return true
}

// common is the set of the errors both p and other hold.
func (p proven) common(other proven) proven {
	bits := make([]byte, min(len(p), len(other)))
	for index := range bits {
		bits[index] = p[index] & other[index]
	}
	return proven(strings.TrimRight(string(bits), "\x00"))
}

func (p proven) without(index int) proven {
	if !p.has(index) {
		return p
	}
	bits := []byte(p)
	bits[index/8] &^= 1 << (index % 8)
	return proven(strings.TrimRight(string(bits), "\x00"))
}
