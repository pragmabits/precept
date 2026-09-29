package paircheck

import (
	"fmt"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"
)

type slotKind int

const (
	slotNone slotKind = iota
	slotReceiver
	slotArgument
	slotResult
	// slotCallee is the function a call calls: the value, for the call
	// satisfier.
	slotCallee
)

// slot is where the value sits in a call: the receiver, an argument, or a
// result, counted from zero. The zero slot is one the configuration left for
// the types to decide.
type slot struct {
	kind  slotKind
	index int
}

func parseSlot(text string) (slot, error) {
	switch text {
	case "":
		return slot{}, nil
	case "receiver":
		return slot{kind: slotReceiver}, nil
	}
	word, number, found := strings.Cut(text, " ")
	kinds := map[string]slotKind{"argument": slotArgument, "result": slotResult}
	kind, known := kinds[word]
	index, err := strconv.Atoi(number)
	if !found || !known || err != nil || index < 0 {
		return slot{}, fmt.Errorf("%w: %q", ErrInvalidSlot, text)
	}
	return slot{kind: kind, index: index}, nil
}

// valueAt is the value a call carries in this slot, or nil when the call does
// not hold it: a result nobody assigned, or a result of a deferred call.
func (s slot) valueAt(instruction ssa.CallInstruction) ssa.Value {
	common := instruction.Common()
	switch s.kind {
	case slotReceiver:
		return receiverOf(common)
	case slotArgument:
		return argumentOf(common, s.index)
	case slotResult:
		if call, ok := instruction.(*ssa.Call); ok {
			return resultOf(call, s.index)
		}
	case slotCallee:
		if !common.IsInvoke() {
			return common.Value
		}
	}
	return nil
}

func receiverOf(common *ssa.CallCommon) ssa.Value {
	if common.IsInvoke() {
		return common.Value
	}
	if receiver, ok := boundReceiver(common.Value); ok {
		return receiver
	}
	if methods, all := keptMethods(common.Value); all && len(methods) == 1 {
		receiver, _ := boundReceiver(methods[0])
		return receiver
	}
	if common.Signature().Recv() == nil || len(common.Args) == 0 {
		return nil
	}
	return common.Args[0]
}

// boundReceiver is the receiver a method value holds: the one value bound by
// the closure SSA builds around a method, which calls it on that receiver.
func boundReceiver(value ssa.Value) (ssa.Value, bool) {
	closure, ok := value.(*ssa.MakeClosure)
	if !ok || len(closure.Bindings) != 1 {
		return nil, false
	}
	function, ok := closure.Fn.(*ssa.Function)
	if !ok {
		return nil, false
	}
	method, ok := function.Object().(*types.Func)
	return closure.Bindings[0], ok && method.Signature().Recv() != nil
}

// keptMethods are the method values a function value read back from memory may
// be: those its function stored where the value may be read from, a field or a
// map entry. They are all it can be when every such store holds a method value
// and comes before the read in its block, with no call between that could
// store another.
func keptMethods(value ssa.Value) ([]*ssa.MakeClosure, bool) {
	read, ok := value.(ssa.Instruction)
	if !ok || !readBack(value) {
		return nil, false
	}
	stores, kept := storedWhere(value, read.Parent())
	var methods []*ssa.MakeClosure
	all := len(stores) > 0
	for index, store := range stores {
		closure, bound := boundMethod(kept[index])
		if bound {
			methods = append(methods, closure)
		}
		all = all && bound && precedes(store, read)
	}
	return methods, all
}

// storedWhere lists the stores of function into the places value may be read
// from, and what each one stores.
func storedWhere(value ssa.Value, function *ssa.Function) ([]ssa.Instruction, []ssa.Value) {
	var stores []ssa.Instruction
	var kept []ssa.Value
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			stored, same := keptAt(value, instruction)
			if stored != nil && same != sameNo {
				stores = append(stores, instruction)
				kept = append(kept, stored)
			}
		}
	}
	return stores, kept
}

// precedes reports whether store comes before read in the block of read, with
// no call between.
func precedes(store, read ssa.Instruction) bool {
	if store.Block() != read.Block() {
		return false
	}
	stored := false
	for _, instruction := range read.Block().Instrs {
		switch instruction.(type) {
		case *ssa.Call, *ssa.Go, *ssa.RunDefers:
			if stored && instruction != read {
				return false
			}
		}
		if instruction == store {
			stored = true
		}
		if instruction == read {
			return stored
		}
	}
	return false
}

// boundMethod is value when it is a method value.
func boundMethod(value ssa.Value) (*ssa.MakeClosure, bool) {
	closure, ok := value.(*ssa.MakeClosure)
	if !ok {
		return nil, false
	}
	_, bound := boundReceiver(closure)
	return closure, bound
}

// readBack reports whether value is read from memory: a load, or an entry of a
// map.
func readBack(value ssa.Value) bool {
	switch read := value.(type) {
	case *ssa.UnOp:
		return read.Op == token.MUL
	case *ssa.Lookup:
		return !read.CommaOk
	}
	return false
}

// keptAt is the value instruction stores where value is read from, a field or
// a map entry, and whether that is the same place.
func keptAt(value ssa.Value, instruction ssa.Instruction) (ssa.Value, sameness) {
	switch read := value.(type) {
	case *ssa.UnOp:
		store, ok := instruction.(*ssa.Store)
		if !ok {
			return nil, sameNo
		}
		return store.Val, sameLocation(read.X, store.Addr)
	case *ssa.Lookup:
		update, ok := instruction.(*ssa.MapUpdate)
		if !ok {
			return nil, sameNo
		}
		return update.Value, sameEntry(read, update)
	}
	return nil, sameNo
}

func argumentOf(common *ssa.CallCommon, index int) ssa.Value {
	if !common.IsInvoke() && common.Signature().Recv() != nil {
		index++
	}
	if index >= len(common.Args) {
		return nil
	}
	return common.Args[index]
}

func resultOf(call *ssa.Call, index int) ssa.Value {
	if call.Common().Signature().Results().Len() == 1 {
		return call
	}
	referrers := call.Referrers()
	if referrers == nil {
		return nil
	}
	for _, referrer := range *referrers {
		if extract, ok := referrer.(*ssa.Extract); ok && extract.Index == index {
			return extract
		}
	}
	return nil
}
