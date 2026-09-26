package paircheck

import (
	"fmt"
	"go/types"
	"slices"

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

// protocol is a rule after validation. The failures are the functions whose
// call returns a non-nil error, which an on-success protocol reads a return by.
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
	onSuccess    bool
	failures     []qualifiedName
}

// withinModule is every protocol with its relative names resolved against
// module, the path of the module the analyzed code belongs to.
func withinModule(protocols []protocol, module string) ([]protocol, error) {
	resolved := make([]protocol, 0, len(protocols))
	for _, current := range protocols {
		within, err := current.within(module)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", current.id, err)
		}
		resolved = append(resolved, within)
	}
	return resolved, nil
}

// within is p with its relative names resolved against module. It never
// writes to p, which every pass shares.
func (p protocol) within(module string) (protocol, error) {
	trigger, err := p.trigger.name.within(module)
	if err != nil {
		return protocol{}, err
	}
	p.trigger.name = trigger
	satisfiers := slices.Clone(p.satisfiers)
	for index, satisfier := range satisfiers {
		if satisfier.called {
			continue
		}
		satisfiers[index].name, err = satisfier.name.within(module)
		if err != nil {
			return protocol{}, err
		}
	}
	p.satisfiers = satisfiers
	p.failures, err = namesWithin(p.failures, module)
	if err != nil {
		return protocol{}, fmt.Errorf("failures: %w", err)
	}
	return p, nil
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
