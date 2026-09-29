package paircheck

import (
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/ssa"
)

// load is the step that reads through a pointer.
const load = "*"

// sameness answers whether two values are the same: yes, no, or unknown.
type sameness int

const (
	sameUnknown sameness = iota
	sameYes
	sameNo
)

// path is how a value is reached: from a root, through fields and loads. Two
// SSA values with one path hold the same thing, which is what makes each
// `s.res` in a function, a new FieldAddr and a new load every time, the same
// value.
type path struct {
	root  ssa.Value
	steps string
	loads bool
}

// compare answers whether first and second are the same value. It says no only
// for distinct fresh roots reached without a load: a load reads memory
// somebody else may have written the same pointer into.
func (b binding) compare(first, second ssa.Value) sameness {
	if first == nil || second == nil {
		return sameUnknown
	}
	return b.comparePaths(pathOf(first), pathOf(second))
}

func (b binding) comparePaths(firstPath, secondPath path) sameness {
	if firstPath == secondPath {
		return sameYes
	}
	if firstPath.loads || secondPath.loads || firstPath.root == secondPath.root {
		return sameUnknown
	}
	if b.fresh(firstPath.root) && b.fresh(secondPath.root) {
		return sameNo
	}
	return sameUnknown
}

// satisfied answers whether the value the satisfier at index carries, reached
// along carried, is the value reached along opened. Under the call satisfier,
// a function the trigger returned is not a parameter, which the function held
// before the trigger ran.
func (b binding) satisfied(index int, carried, opened path) sameness {
	same := b.comparePaths(carried, opened)
	if same != sameUnknown || !b.protocol.satisfiers[index].called {
		return same
	}
	if b.freshBeside(opened, carried) || b.freshBeside(carried, opened) {
		return sameNo
	}
	return same
}

// freshBeside reports whether made is a function a trigger returned and other
// a parameter of the function the trigger was called in.
func (b binding) freshBeside(made, other path) bool {
	call, ok := made.root.(ssa.Instruction)
	return ok && b.fresh(made.root) && !made.loads && received(other, call.Parent())
}

// received reports whether reached is a parameter of within, as it was passed:
// the parameter, or a load of the variable a closure captures it in, when
// nothing else is ever stored there. A parameter of a closure is not one: a
// call binds it after the trigger ran.
func received(reached path, within *ssa.Function) bool {
	switch root := reached.root.(type) {
	case *ssa.Parameter:
		return root.Parent() == within && reached.steps == ""
	case *ssa.Alloc:
		return root.Parent() == within && reached.steps == load && holdsParameter(root)
	}
	return false
}

// holdsParameter reports whether variable only ever holds a parameter: its
// function stores nothing else into it, and no closure capturing it stores
// into it at all.
func holdsParameter(variable *ssa.Alloc) bool {
	referrers := variable.Referrers()
	if referrers == nil {
		return false
	}
	stored := false
	for _, referrer := range *referrers {
		switch use := referrer.(type) {
		case *ssa.Store:
			if _, parameter := use.Val.(*ssa.Parameter); !parameter || use.Addr != variable {
				return false
			}
			stored = true
		case *ssa.UnOp:
		case *ssa.MakeClosure:
			if storesCaptured(use, variable) {
				return false
			}
		default:
			return false
		}
	}
	return stored
}

// storesCaptured reports whether closure stores into variable, which it
// captures.
func storesCaptured(closure *ssa.MakeClosure, variable ssa.Value) bool {
	function, ok := closure.Fn.(*ssa.Function)
	if !ok {
		return true
	}
	for index, binding := range closure.Bindings {
		if binding != variable || index >= len(function.FreeVars) {
			continue
		}
		referrers := function.FreeVars[index].Referrers()
		if referrers == nil {
			continue
		}
		for _, referrer := range *referrers {
			if _, store := referrer.(*ssa.Store); store {
				return true
			}
		}
	}
	return false
}

// fresh reports whether root is an object no other value can already hold: an
// allocation of this function, or the value a trigger call returned.
func (b binding) fresh(root ssa.Value) bool {
	switch typed := root.(type) {
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeClosure:
		return true
	case *ssa.Call:
		return b.returnedByTrigger(typed, typed)
	case *ssa.Extract:
		call, ok := typed.Tuple.(*ssa.Call)
		return ok && b.returnedByTrigger(call, typed)
	}
	return false
}

func (b binding) returnedByTrigger(call *ssa.Call, value ssa.Value) bool {
	return b.triggerSlot.kind == slotResult &&
		b.protocol.opens(call.Common()) &&
		b.triggerSlot.valueAt(call) == value
}

// sameLocation answers whether two addresses reach the same memory: yes along
// one path; no along paths that part at a field with no load before, or from
// two distinct allocations; unknown otherwise.
func sameLocation(first, second ssa.Value) sameness {
	firstPath, secondPath := pathOf(first), pathOf(second)
	switch {
	case firstPath == secondPath:
		return sameYes
	case firstPath.loads || secondPath.loads:
		return sameUnknown
	case firstPath.root == secondPath.root:
		return sameNo
	case allocated(firstPath.root) && allocated(secondPath.root):
		return sameNo
	}
	return sameUnknown
}

// sameEntry answers whether a lookup reads the entry an update writes: the
// same map, by a key both spell alike.
func sameEntry(lookup *ssa.Lookup, update *ssa.MapUpdate) sameness {
	if !types.Identical(lookup.X.Type().Underlying(), update.Map.Type().Underlying()) {
		return sameNo
	}
	maps := sameLocation(lookup.X, update.Map)
	if maps == sameNo {
		return sameNo
	}
	keys := sameKey(lookup.Index, update.Key)
	switch {
	case keys == sameNo:
		return sameNo
	case maps == sameYes && keys == sameYes:
		return sameYes
	}
	return sameUnknown
}

// sameKey compares two keys of a map: one value, or two constants of one
// type. Keys of two types belong to two maps, which is for the maps to tell.
func sameKey(first, second ssa.Value) sameness {
	if first == second {
		return sameYes
	}
	firstConstant, firstOK := first.(*ssa.Const)
	secondConstant, secondOK := second.(*ssa.Const)
	if !firstOK || !secondOK || firstConstant.Value == nil || secondConstant.Value == nil ||
		!types.Identical(first.Type(), second.Type()) {
		return sameUnknown
	}
	if constant.Compare(firstConstant.Value, token.EQL, secondConstant.Value) {
		return sameYes
	}
	return sameNo
}

// allocated reports whether root is memory this function made.
func allocated(root ssa.Value) bool {
	switch root.(type) {
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan, *ssa.MakeClosure:
		return true
	}
	return false
}

// homesOf lists the variables value was stored into.
func homesOf(value ssa.Value) []ssa.Value {
	if value == nil || value.Referrers() == nil {
		return nil
	}
	var homes []ssa.Value
	for _, referrer := range *value.Referrers() {
		if store, ok := referrer.(*ssa.Store); ok && store.Val == value {
			homes = append(homes, store.Addr)
		}
	}
	return homes
}

func pathOf(value ssa.Value) path {
	return walk(value, make(map[*ssa.Phi]bool))
}

func walk(value ssa.Value, visiting map[*ssa.Phi]bool) path {
	if inner, step, ok := unwrap(value); ok {
		reached := walk(inner, visiting)
		reached.steps += step
		reached.loads = reached.loads || step == load
		return reached
	}
	if phi, ok := value.(*ssa.Phi); ok {
		return walkPhi(phi, visiting)
	}
	return path{root: value}
}

// walkPhi is the path every edge of phi agrees on, or phi itself as a root
// when the edges disagree. An edge that loops back to phi says nothing.
func walkPhi(phi *ssa.Phi, visiting map[*ssa.Phi]bool) path {
	if visiting[phi] {
		return path{}
	}
	visiting[phi] = true
	defer delete(visiting, phi)
	var agreed path
	for _, edge := range phi.Edges {
		reached := walk(edge, visiting)
		switch {
		case reached.root == nil:
			continue
		case agreed.root == nil:
			agreed = reached
		case agreed != reached:
			return path{root: phi}
		}
	}
	if agreed.root == nil {
		return path{root: phi}
	}
	return agreed
}

// unwrap is the value inside value, and the step that leads from one to the
// other. A conversion keeps the value and adds no step.
func unwrap(value ssa.Value) (ssa.Value, string, bool) {
	switch typed := value.(type) {
	case *ssa.FieldAddr:
		return typed.X, "&" + strconv.Itoa(typed.Field), true
	case *ssa.Field:
		return typed.X, "." + strconv.Itoa(typed.Field), true
	case *ssa.UnOp:
		return typed.X, load, typed.Op == token.MUL
	case *ssa.MakeInterface:
		return typed.X, "", true
	case *ssa.ChangeInterface:
		return typed.X, "", true
	case *ssa.ChangeType:
		return typed.X, "", true
	case *ssa.TypeAssert:
		return typed.X, "", true
	}
	return nil, "", false
}
