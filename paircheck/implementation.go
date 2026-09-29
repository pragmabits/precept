package paircheck

import (
	"errors"
	"fmt"
	"go/types"
	"slices"
)

var (
	ErrUnknownType             = errors.New("no such type")
	ErrImplementsNone          = errors.New("the type implements no interface the rule names")
	ErrAmbiguousImplementation = errors.New(
		"the type has a method of the rule on a receiver it does not implement",
	)
)

// implemented is current applied to each of its implementations the pass sees:
// the methods of the type in place of those of the interfaces it implements,
// or, where the pass does not see an interface, of those it has. A type the
// pass does not see is left out, since the pass cannot call its methods. A
// type missing from a package the pass sees, or one that implements none of
// the interfaces the pass sees them all of, is an error.
func implemented(current protocol, visible map[string]*types.Package) ([]protocol, error) {
	var applied []protocol
	for _, each := range current.implementations {
		typeName, found, err := lookupType(visible, each.name)
		if err == nil && found {
			derived, swapped := current.appliedTo(typeName, each, visible)
			switch {
			case swapped:
				applied = append(applied, derived)
			case current.seesInterfaces(visible):
				err = ErrImplementsNone
			}
		}
		if err != nil {
			return nil, fmt.Errorf("rule %q: implementation %s: %w", current.id, each.name, err)
		}
	}
	return applied, nil
}

// lookupType finds the type name names, if the pass sees its package.
func lookupType(
	visible map[string]*types.Package,
	name qualifiedName,
) (*types.TypeName, bool, error) {
	declaring, found := visible[name.path]
	if !found {
		return nil, false, nil
	}
	typeName, ok := declaring.Scope().Lookup(name.name).(*types.TypeName)
	if !ok {
		return nil, false, ErrUnknownType
	}
	return typeName, true, nil
}

// appliedTo is p with the methods of typeName in place of those it stands for,
// reporting with the id of the implementation, and whether any was replaced.
func (p protocol) appliedTo(
	typeName *types.TypeName,
	each implementation,
	visible map[string]*types.Package,
) (protocol, bool) {
	derived := p
	derived.id = each.id
	derived.label = fmt.Sprintf("%q: implementation %s", p.id, each.name)
	derived.implementations = nil
	trigger, swapped := swap(p.trigger, typeName, visible)
	derived.trigger = trigger
	derived.satisfiers = slices.Clone(p.satisfiers)
	for index, satisfier := range derived.satisfiers {
		replaced, ok := swap(satisfier, typeName, visible)
		derived.satisfiers[index] = replaced
		swapped = swapped || ok
	}
	return derived, swapped
}

// seesInterfaces reports whether the pass sees the receiver of every method p
// names, which settles what a type implements.
func (p protocol) seesInterfaces(visible map[string]*types.Package) bool {
	for _, current := range append([]side{p.trigger}, p.satisfiers...) {
		if current.called || current.name.receiver == "" {
			continue
		}
		if _, seen := interfaceOf(visible, current.name); !seen {
			return false
		}
	}
	return true
}

// swap is current with the method of typeName in place of its own, when
// typeName stands for the receiver.
func swap(current side, typeName *types.TypeName, visible map[string]*types.Package) (side, bool) {
	if current.called || current.name.receiver == "" || !stands(typeName, current.name, visible) {
		return current, false
	}
	current.name = qualifiedName{
		path:     typeName.Pkg().Path(),
		receiver: typeName.Name(),
		name:     current.name.name,
	}
	return current, true
}

// stands reports whether typeName stands for the receiver of method: it
// implements the interface method is declared on, or, where the pass does not
// see that interface, it has a method of that name.
func stands(
	typeName *types.TypeName,
	method qualifiedName,
	visible map[string]*types.Package,
) bool {
	if contract, seen := interfaceOf(visible, method); seen {
		return contract != nil && implements(typeName.Type(), contract)
	}
	return hasMethod(typeName, method.name)
}

// interfaceOf is the interface the receiver of method is, and whether the pass
// sees its package. It is nil when that receiver is no interface.
func interfaceOf(
	visible map[string]*types.Package,
	method qualifiedName,
) (*types.Interface, bool) {
	declaring, found := visible[method.path]
	if !found {
		return nil, false
	}
	receiver, ok := declaring.Scope().Lookup(method.receiver).(*types.TypeName)
	if !ok {
		return nil, true
	}
	contract, _ := receiver.Type().Underlying().(*types.Interface)
	return contract, true
}

// implements reports whether a value of concrete, or a pointer to one,
// implements contract.
func implements(concrete types.Type, contract *types.Interface) bool {
	return types.Implements(concrete, contract) ||
		types.Implements(types.NewPointer(concrete), contract)
}

func hasMethod(typeName *types.TypeName, name string) bool {
	object, _, _ := types.LookupFieldOrMethod(
		types.NewPointer(typeName.Type()),
		false,
		typeName.Pkg(),
		name,
	)
	_, method := object.(*types.Func)
	return method
}

// implementationProblems is what Validate finds wrong in the implementations
// of current, over one load of every package they name: a type that does not
// exist, one with a method of the rule on a receiver it does not stand for,
// one that implements none of its interfaces, and one the rule does not bind
// on.
func implementationProblems(current protocol, visible map[string]*types.Package) []error {
	var problems []error
	for _, each := range current.implementations {
		if err := implementationProblem(current, each, visible); err != nil {
			problems = append(problems, fmt.Errorf(
				"rule %q: implementation %s: %w",
				current.id,
				each.name,
				err,
			))
		}
	}
	return problems
}

func implementationProblem(
	current protocol,
	each implementation,
	visible map[string]*types.Package,
) error {
	typeName, found, err := lookupType(visible, each.name)
	switch {
	case err != nil:
		return err
	case !found:
		return ErrUnknownType
	}
	if method, ok := current.ambiguous(typeName, visible); ok {
		return fmt.Errorf("%w: %s", ErrAmbiguousImplementation, method)
	}
	derived, swapped := current.appliedTo(typeName, each, visible)
	if !swapped {
		return ErrImplementsNone
	}
	_, err = resolve(derived, visible, true)
	return err
}

// ambiguous is a method p names that typeName has a method of the same name
// as, without standing for its receiver: a pass that does not see the receiver
// would put the type's method in its place.
func (p protocol) ambiguous(
	typeName *types.TypeName,
	visible map[string]*types.Package,
) (qualifiedName, bool) {
	for _, current := range append([]side{p.trigger}, p.satisfiers...) {
		if current.called || current.name.receiver == "" {
			continue
		}
		if hasMethod(typeName, current.name.name) && !stands(typeName, current.name, visible) {
			return current.name, true
		}
	}
	return qualifiedName{}, false
}
