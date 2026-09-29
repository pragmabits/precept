package paircheck

import (
	"fmt"
	"go/types"
	"slices"
	"strconv"
	"strings"

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

// implementation is a concrete type a protocol applies to as well: its name,
// in the name's place for a type, and the id of what it reports.
type implementation struct {
	name qualifiedName
	id   string
}

// protocol is a rule after validation. The failures are the functions whose
// call returns a non-nil error, which an on-success protocol reads a return by.
// A protocol an implementation applies the rule to carries the label its
// errors are told by.
type protocol struct {
	id           string
	label        string
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

	implementations []implementation
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
	implementations := slices.Clone(p.implementations)
	for index, each := range implementations {
		implementations[index].name, err = each.name.within(module)
		if err != nil {
			return protocol{}, fmt.Errorf("implementation %d: %w", index, err)
		}
	}
	p.implementations = implementations
	return p, nil
}

// key tells the protocol apart across the passes of one run: by its id and the
// names it resolved to, which differ where a relative name met another module.
// A summary of a function is filed under it.
func (p protocol) key() string {
	names := []string{p.id, p.trigger.name.String()}
	for _, satisfier := range p.satisfiers {
		names = append(names, satisfier.display()+" "+satisfier.name.String())
	}
	return strings.Join(names, " ")
}

// described is how an error tells the protocol: by the id of its rule, and the
// implementation it applies the rule to.
func (p protocol) described() string {
	if p.label != "" {
		return p.label
	}
	return strconv.Quote(p.id)
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

// named reports whether function is the trigger or a satisfier of p.
func (p protocol) named(function *types.Func) bool {
	if p.trigger.name.matches(function) {
		return true
	}
	_, satisfier := p.satisfierNamed(function)
	return satisfier
}

// relative reports whether p names a function, a method or a type relative to
// the module.
func (p protocol) relative() bool {
	if p.trigger.name.relative {
		return true
	}
	for _, satisfier := range p.satisfiers {
		if !satisfier.called && satisfier.name.relative {
			return true
		}
	}
	return slices.ContainsFunc(p.implementations, func(each implementation) bool {
		return each.name.relative
	})
}

// absolute is p without the failures named relative to the module.
func (p protocol) absolute() protocol {
	p.failures = slices.DeleteFunc(slices.Clone(p.failures), func(name qualifiedName) bool {
		return name.relative
	})
	return p
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
