package paircheck

import (
	"fmt"
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
