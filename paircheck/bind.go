package paircheck

import (
	"errors"
	"fmt"
	"go/types"

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

// candidate is a slot that can carry the value, with the type it has there.
type candidate struct {
	slot      slot
	valueType types.Type
}

// binding is a protocol resolved in one pass: the slot carrying the value on
// each side, deduced from the types of the functions the pass can see, and the
// result through which the trigger reports failure. A satisfier the pass
// cannot see keeps the zero slot; the pass cannot call it.
type binding struct {
	protocol       protocol
	triggerSlot    slot
	satisfierSlots []slot
	satisfiers     []*types.Func
	failure        int
}

// bind resolves current against the packages visible from a pass. It reports
// false when the trigger is not visible, or when the types do not settle one
// slot on each visible side.
func bind(current protocol, visible map[string]*types.Package) (binding, bool) {
	resolved, err := resolve(current, visible, false)
	return resolved, err == nil
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
	valueType, err := linkingType(opening, closing, seen)
	if err != nil {
		return binding{}, err
	}
	resolved, err := settle(current, valueType, opening, closing, seen)
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
	valueType types.Type,
	opening []candidate,
	closing [][]candidate,
	seen []bool,
) (binding, error) {
	triggerSlot, err := only(opening, valueType)
	if err != nil {
		return binding{}, fmt.Errorf("trigger %s: %w", current.trigger.name, err)
	}
	resolved := binding{
		protocol:       current,
		triggerSlot:    triggerSlot,
		satisfierSlots: make([]slot, len(closing)),
	}
	for index, offered := range closing {
		if !seen[index] {
			continue
		}
		resolved.satisfierSlots[index], err = only(offered, valueType)
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
// the pass can see.
func linkingType(opening []candidate, closing [][]candidate, seen []bool) (types.Type, error) {
	var linking []types.Type
	for _, offered := range opening {
		if containsType(linking, offered.valueType) {
			continue
		}
		if offeredByAll(closing, seen, offered.valueType) {
			linking = append(linking, offered.valueType)
		}
	}
	switch len(linking) {
	case 0:
		return nil, ErrNoLinkingType
	case 1:
		return linking[0], nil
	}
	return nil, ErrAmbiguousSlot
}

func offeredByAll(closing [][]candidate, seen []bool, valueType types.Type) bool {
	for index, offered := range closing {
		if seen[index] && !offers(offered, valueType) {
			return false
		}
	}
	return true
}

func offers(offered []candidate, valueType types.Type) bool {
	_, found := find(offered, valueType)
	return found
}

// only is the slot of the one candidate of valueType.
func only(offered []candidate, valueType types.Type) (slot, error) {
	position, found := find(offered, valueType)
	if !found {
		return slot{}, ErrNoLinkingType
	}
	for _, other := range offered[position+1:] {
		if sameType(other.valueType, valueType) {
			return slot{}, ErrAmbiguousSlot
		}
	}
	return offered[position].slot, nil
}

func find(offered []candidate, valueType types.Type) (int, bool) {
	for position, current := range offered {
		if sameType(current.valueType, valueType) {
			return position, true
		}
	}
	return 0, false
}

func containsType(list []types.Type, valueType types.Type) bool {
	for _, current := range list {
		if sameType(current, valueType) {
			return true
		}
	}
	return false
}

// candidates lists the slots of function that can carry the value: the
// receiver and the parameters, and for a trigger the results but a final
// error.
func candidates(function *types.Func, withResults bool) []candidate {
	signature := function.Signature()
	offered := make([]candidate, 0, signature.Params().Len()+signature.Results().Len()+1)
	if receiver := signature.Recv(); receiver != nil {
		offered = append(offered, candidate{slot{kind: slotReceiver}, receiver.Type()})
	}
	for index := range signature.Params().Len() {
		parameter := signature.Params().At(index)
		offered = append(offered, candidate{slot{kind: slotArgument, index: index}, parameter.Type()})
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
		offered = append(offered, candidate{slot{kind: slotResult, index: index}, result.Type()})
	}
	return offered
}

func restrict(offered []candidate, wanted slot) []candidate {
	if wanted.kind == slotNone {
		return offered
	}
	kept := make([]candidate, 0, 1)
	for _, current := range offered {
		if current.slot == wanted {
			kept = append(kept, current)
		}
	}
	return kept
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
