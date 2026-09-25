package paircheck

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// side is one end of a protocol: the function or method, and the slot the
// configuration pinned the value to, if it did.
type side struct {
	name qualifiedName
	slot slot
}

// protocol is a rule after validation.
type protocol struct {
	id          string
	trigger     side
	satisfiers  []side
	openOnError bool
	coverage    Coverage
	escapes     escapes
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
	for index, satisfier := range p.satisfiers {
		if satisfier.name.matches(callee) {
			return index, true
		}
	}
	return 0, false
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
