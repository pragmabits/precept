package paircheck

import (
	"errors"
	"fmt"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

var (
	ErrUnknownFunction    = errors.New("no such function or method")
	ErrSlotNotInSignature = errors.New("slot is not in the signature")
	ErrNoLinkingType      = errors.New("no type links the trigger to every satisfier")
	ErrAmbiguousSlot      = errors.New(
		"more than one slot has the type that links the trigger to the satisfiers",
	)
)

var errorType = types.Universe.Lookup("error").Type()

// candidate is a slot that can carry the value, with the type it has there,
// and whether the rule wrote that slot. A called candidate is the call
// satisfier's: it takes any function.
type candidate struct {
	slot      slot
	valueType types.Type
	written   bool
	called    bool
}

// binding is a protocol resolved in one pass: the slot carrying the value on
// each side, deduced from the types of the functions the pass can see, and the
// result through which the trigger reports failure. A satisfier the pass
// cannot see keeps the zero slot; the pass cannot call it.
type binding struct {
	protocol       protocol
	valueType      types.Type
	triggerSlot    slot
	satisfierSlots []slot
	satisfiers     []*types.Func
	failure        int
}

// bind resolves current against the packages visible from a pass. A pass that
// sees only some of the functions current names skips a rule it cannot
// resolve, since a satisfier it does not see may be what settles the slot. A
// pass that sees all of them knows what Validate knows, and a rule it cannot
// resolve is an error.
func bind(current protocol, visible map[string]*types.Package) (binding, bool, error) {
	resolved, err := resolve(current, visible, false)
	if err == nil {
		return resolved, true, nil
	}
	if !seesAll(visible, current) {
		return binding{}, false, nil
	}
	return binding{}, false, fmt.Errorf("rule %q: %w", current.id, err)
}

// seesAll reports whether every function current names is in the visible
// packages.
func seesAll(visible map[string]*types.Package, current protocol) bool {
	if lookupFunc(visible, current.trigger.name) == nil {
		return false
	}
	for _, satisfier := range current.satisfiers {
		if !satisfier.called && lookupFunc(visible, satisfier.name) == nil {
			return false
		}
	}
	return true
}

// resolve binds current against the visible packages, or says why it cannot.
// Strict, a satisfier it cannot find is an error; otherwise it is one the pass
// cannot see, and cannot call.
func resolve(
	current protocol,
	visible map[string]*types.Package,
	strict bool,
) (binding, error) {
	opening, trigger, err := offeredBy(visible, current.trigger, true)
	if err != nil {
		return binding{}, fmt.Errorf("trigger: %w", err)
	}
	closing := make([][]candidate, len(current.satisfiers))
	seen := make([]bool, len(current.satisfiers))
	satisfiers := make([]*types.Func, len(current.satisfiers))
	for index, satisfier := range current.satisfiers {
		if satisfier.called {
			seen[index] = true
			closing[index] = []candidate{{slot: slot{kind: slotCallee}, called: true}}
			continue
		}
		offered, function, err := offeredBy(visible, satisfier, false)
		if errors.Is(err, ErrUnknownFunction) && !strict {
			continue
		}
		if err != nil {
			return binding{}, fmt.Errorf("satisfier %d: %w", index, err)
		}
		seen[index] = true
		closing[index] = offered
		satisfiers[index] = function
	}
	link, err := linkingType(opening, closing, seen)
	if err != nil {
		return binding{}, err
	}
	resolved, err := settle(current, link, opening, closing, seen)
	if err != nil {
		return binding{}, err
	}
	resolved.satisfiers = satisfiers
	resolved.failure = -1
	if !current.openOnError {
		resolved.failure = failureIndex(trigger)
	}
	return resolved, nil
}

// offeredBy finds the function a side names, and the slots it offers for the
// value.
func offeredBy(
	visible map[string]*types.Package,
	wanted side,
	withResults bool,
) ([]candidate, *types.Func, error) {
	function := lookupFunc(visible, wanted.name)
	if function == nil {
		return nil, nil, fmt.Errorf("%s: %w", wanted.name, ErrUnknownFunction)
	}
	offered := restrict(candidates(function, withResults), wanted.slot)
	if wanted.slot.kind != slotNone && len(offered) == 0 {
		return nil, nil, fmt.Errorf("%s: %w", wanted.name, ErrSlotNotInSignature)
	}
	return offered, function, nil
}

func settle(
	current protocol,
	link candidate,
	opening []candidate,
	closing [][]candidate,
	seen []bool,
) (binding, error) {
	triggerSlot, err := only(opening, link)
	if err != nil {
		return binding{}, fmt.Errorf("trigger %s: %w", current.trigger.name, err)
	}
	resolved := binding{
		protocol:       current,
		valueType:      link.valueType,
		triggerSlot:    triggerSlot,
		satisfierSlots: make([]slot, len(closing)),
	}
	for index, offered := range closing {
		if !seen[index] {
			continue
		}
		resolved.satisfierSlots[index], err = only(offered, link)
		if err != nil {
			return binding{}, fmt.Errorf("satisfier %s: %w", current.satisfiers[index].name, err)
		}
	}
	return resolved, nil
}

// obligations lists the trigger calls in function, each with the value it
// opens an obligation on.
func (b binding) obligations(function *ssa.Function) []obligation {
	var opened []obligation
	for _, block := range function.Blocks {
		for index, instruction := range block.Instrs {
			call, ok := instruction.(*ssa.Call)
			if !ok || !b.protocol.opens(call.Common()) {
				continue
			}
			value := b.triggerSlot.valueAt(call)
			opened = append(opened, obligation{
				call:    call,
				index:   index,
				value:   value,
				homes:   homesOf(value),
				failure: failureOf(b.failureAt(call)),
			})
		}
	}
	return opened
}

// satisfierOf reports which satisfier common calls: by its name, or through an
// interface whose method the satisfier's receiver implements, which dispatches
// to the satisfier when the interface holds a value of that receiver.
func (b binding) satisfierOf(common *ssa.CallCommon) (int, bool) {
	if index, ok := b.protocol.satisfiedBy(common); ok {
		return index, true
	}
	if index, ok := b.calledBy(common); ok {
		return index, true
	}
	if !common.IsInvoke() {
		return 0, false
	}
	contract, ok := common.Value.Type().Underlying().(*types.Interface)
	if !ok {
		return 0, false
	}
	for index, satisfier := range b.satisfiers {
		if dispatchesTo(common.Method, contract, satisfier) {
			return index, true
		}
	}
	return 0, false
}

// calledBy reports which satisfier common is when it is a call of a function
// value of the value's own type: the call satisfier. A function value of
// another type, such as a callback, is not the value.
func (b binding) calledBy(common *ssa.CallCommon) (int, bool) {
	if common.IsInvoke() || common.StaticCallee() != nil {
		return 0, false
	}
	if !types.Identical(common.Value.Type(), b.valueType) {
		return 0, false
	}
	index := slices.IndexFunc(b.protocol.satisfiers, func(current side) bool {
		return current.called
	})
	return index, index >= 0
}

func isFunction(valueType types.Type) bool {
	if valueType == nil {
		return false
	}
	_, ok := valueType.Underlying().(*types.Signature)
	return ok
}

func dispatchesTo(method *types.Func, contract *types.Interface, satisfier *types.Func) bool {
	if satisfier == nil || satisfier.Name() != method.Name() {
		return false
	}
	receiver := satisfier.Signature().Recv()
	return receiver != nil && types.Implements(receiver.Type(), contract)
}

// failureAt is the error a trigger call returned, or nil when it returns none,
// when nobody holds it, or when the protocol opens on error.
func (b binding) failureAt(call *ssa.Call) ssa.Value {
	if b.failure < 0 {
		return nil
	}
	return resultOf(call, b.failure)
}

// linkingType is the one type offered by the trigger and by every satisfier
// the pass can see, as the trigger's candidate that carries it.
func linkingType(opening []candidate, closing [][]candidate, seen []bool) (candidate, error) {
	var linking []candidate
	for _, offered := range opening {
		if containsLink(linking, offered) {
			continue
		}
		if offeredByAll(closing, seen, offered) {
			linking = append(linking, offered)
		}
	}
	switch len(linking) {
	case 0:
		return candidate{}, noLink(opening, closing)
	case 1:
		return linking[0], nil
	}
	return candidate{}, ErrAmbiguousSlot
}

// noLink is ErrNoLinkingType, saying why when a type parameter sits in a slot
// the rule did not write.
func noLink(opening []candidate, closing [][]candidate) error {
	unwritten := func(offered candidate) bool {
		return !offered.written && mentionsTypeParameter(offered.valueType)
	}
	if slices.ContainsFunc(opening, unwritten) ||
		slices.ContainsFunc(slices.Concat(closing...), unwritten) {
		return fmt.Errorf(
			"%w: a type parameter links only through a slot written on each side",
			ErrNoLinkingType,
		)
	}
	return ErrNoLinkingType
}

func offeredByAll(closing [][]candidate, seen []bool, link candidate) bool {
	for index, offered := range closing {
		if seen[index] && !offers(offered, link) {
			return false
		}
	}
	return true
}

func offers(offered []candidate, link candidate) bool {
	_, found := find(offered, link)
	return found
}

// only is the slot of the one candidate that links with link.
func only(offered []candidate, link candidate) (slot, error) {
	position, found := find(offered, link)
	if !found {
		return slot{}, ErrNoLinkingType
	}
	for _, other := range offered[position+1:] {
		if links(other, link) {
			return slot{}, ErrAmbiguousSlot
		}
	}
	return offered[position].slot, nil
}

func find(offered []candidate, link candidate) (int, bool) {
	for position, current := range offered {
		if links(current, link) {
			return position, true
		}
	}
	return 0, false
}

func containsLink(list []candidate, link candidate) bool {
	for _, current := range list {
		if links(current, link) {
			return true
		}
	}
	return false
}

// links reports whether two candidates can carry the same value: they have the
// same type or, in slots the rule wrote on both sides, types that differ only
// in type parameters that correspond one to one.
func links(first, second candidate) bool {
	switch {
	case first.called:
		return isFunction(second.valueType)
	case second.called:
		return isFunction(first.valueType)
	}
	if first.written && second.written &&
		(mentionsTypeParameter(first.valueType) || mentionsTypeParameter(second.valueType)) {
		return correspond(dereference(first.valueType), dereference(second.valueType))
	}
	return sameType(first.valueType, second.valueType)
}

// candidates lists the slots of function that can carry the value: the
// receiver and the parameters, and for a trigger the results but a final
// error.
func candidates(function *types.Func, withResults bool) []candidate {
	signature := function.Signature()
	offered := make([]candidate, 0, signature.Params().Len()+signature.Results().Len()+1)
	if receiver := signature.Recv(); receiver != nil {
		offered = append(offered, candidate{slot: slot{kind: slotReceiver}, valueType: receiver.Type()})
	}
	for index := range signature.Params().Len() {
		parameter := signature.Params().At(index)
		offered = append(offered, candidate{
			slot:      slot{kind: slotArgument, index: index},
			valueType: parameter.Type(),
		})
	}
	if !withResults {
		return offered
	}
	results := signature.Results()
	count := results.Len()
	if count > 0 && types.Identical(results.At(count-1).Type(), errorType) {
		count--
	}
	for index := range count {
		result := results.At(index)
		offered = append(offered, candidate{
			slot:      slot{kind: slotResult, index: index},
			valueType: result.Type(),
		})
	}
	return offered
}

// restrict keeps the candidates the rule allows: the slot it wrote, or, when it
// wrote none, every candidate but a context.
func restrict(offered []candidate, wanted slot) []candidate {
	if wanted.kind == slotNone {
		return deducible(offered)
	}
	kept := make([]candidate, 0, 1)
	for _, current := range offered {
		if current.slot == wanted {
			current.written = true
			kept = append(kept, current)
		}
	}
	return kept
}

// deducible drops the candidates of type context.Context. A context is passed
// along nearly every call of an API, so it would link most triggers to their
// satisfiers beside the value; a rule on the context itself writes its slots.
func deducible(offered []candidate) []candidate {
	kept := make([]candidate, 0, len(offered))
	for _, current := range offered {
		if !isContext(current.valueType) {
			kept = append(kept, current)
		}
	}
	return kept
}

func isContext(valueType types.Type) bool {
	named, ok := types.Unalias(valueType).(*types.Named)
	if !ok {
		return false
	}
	object := named.Obj()
	return object.Pkg() != nil && object.Pkg().Path() == "context" && object.Name() == "Context"
}

// sameType compares two types without one level of pointer, since a method
// with a value receiver is called on a pointer with no conversion, and a
// generic type by its declaration, since each method of it declares its own
// type parameters.
func sameType(first, second types.Type) bool {
	first, second = dereference(first), dereference(second)
	firstNamed, firstIsNamed := first.(*types.Named)
	secondNamed, secondIsNamed := second.(*types.Named)
	if firstIsNamed && secondIsNamed {
		return firstNamed.Origin().Obj() == secondNamed.Origin().Obj()
	}
	return types.Identical(first, second)
}

func dereference(valueType types.Type) types.Type {
	valueType = types.Unalias(valueType)
	if pointer, ok := valueType.(*types.Pointer); ok {
		return types.Unalias(pointer.Elem())
	}
	return valueType
}

// visiblePackages maps the path of every package reachable from root through
// its imports to the package.
func visiblePackages(root *types.Package) map[string]*types.Package {
	visible := map[string]*types.Package{root.Path(): root}
	pending := []*types.Package{root}
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]
		for _, imported := range current.Imports() {
			if _, known := visible[imported.Path()]; !known {
				visible[imported.Path()] = imported
				pending = append(pending, imported)
			}
		}
	}
	return visible
}

// lookupFunc finds the function or method named by wanted among the visible
// packages, if it is declared where the name says.
func lookupFunc(visible map[string]*types.Package, wanted qualifiedName) *types.Func {
	declaring, found := visible[wanted.path]
	if !found {
		return nil
	}
	if wanted.receiver == "" {
		function, _ := declaring.Scope().Lookup(wanted.name).(*types.Func)
		return function
	}
	typeName, ok := declaring.Scope().Lookup(wanted.receiver).(*types.TypeName)
	if !ok {
		return nil
	}
	object, _, _ := types.LookupFieldOrMethod(typeName.Type(), true, declaring, wanted.name)
	function, ok := object.(*types.Func)
	if !ok || receiverName(function) != wanted.receiver {
		return nil
	}
	return function
}
