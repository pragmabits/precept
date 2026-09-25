package paircheck

import (
	"go/token"
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
