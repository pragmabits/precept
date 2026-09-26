package paircheck

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// side is one end of a protocol: the function or method, and the slot the
// configuration pinned the value to, if it did. A called side is the call
// satisfier: a call of the value itself, which names no function.
type side struct {
	name   qualifiedName
	slot   slot
	called bool
}

// protocol is a rule after validation.
type protocol struct {
	id           string
	trigger      side
	satisfiers   []side
	openOnError  bool
	coverage     Coverage
	escapes      escapes
	requireDefer bool
	idempotent   bool
	deferFirst   bool
}

func (p protocol) opens(common *ssa.CallCommon) bool {
	callee := calleeOf(common)
	return callee != nil && p.trigger.name.matches(callee)
}

// satisfiedBy reports which satisfier common calls, if any.
func (p protocol) satisfiedBy(common *ssa.CallCommon) (int, bool) {
	callee := calleeOf(common)
	if callee == nil {
		return 0, false
	}
	return p.satisfierNamed(callee)
}

// satisfierNamed reports which satisfier names callee, if any.
func (p protocol) satisfierNamed(callee *types.Func) (int, bool) {
	for index, satisfier := range p.satisfiers {
		if !satisfier.called && satisfier.name.matches(callee) {
			return index, true
		}
	}
	return 0, false
}

// display spells the side in a diagnostic.
func (s side) display() string {
	if s.called {
		return "a call"
	}
	return s.name.name
}

func calleeOf(common *ssa.CallCommon) *types.Func {
	if common.IsInvoke() {
		return common.Method
	}
	function := common.StaticCallee()
	if function == nil {
		return nil
	}
	object, _ := function.Object().(*types.Func)
	return object
}
