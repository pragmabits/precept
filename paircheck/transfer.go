package paircheck

import (
	"slices"

	"golang.org/x/tools/go/ssa"
)

// variadic is the comment go/ssa gives the array it builds for the arguments
// of a variadic call.
const variadic = "varargs"

// escapes says through which ways out of a function a value takes its
// obligation along.
type escapes struct {
	returned bool
	stored   bool
	passed   bool
}

// returns reports whether ret hands the value to the caller, when the protocol
// lets a returned value take its obligation along.
func (s search) returns(ret *ssa.Return) bool {
	return s.binding.protocol.escapes.returned && s.anyCarries(ret.Results)
}

// escaped reports whether instruction takes the value out of the function by a
// way that takes the obligation along. A call is not here: whether it hands
// the value over is asked once it is known to be neither a satisfier nor a
// trigger.
func (s search) escaped(instruction ssa.Instruction) bool {
	return s.storedAway(instruction) || s.passedAway(instruction)
}

func (s search) storedAway(instruction ssa.Instruction) bool {
	stored := s.binding.protocol.escapes.stored
	switch typed := instruction.(type) {
	case *ssa.Store:
		return s.keptOutside(typed)
	case *ssa.MapUpdate:
		return stored && (s.carries(typed.Value) || s.carries(typed.Key))
	case *ssa.Send:
		return stored && s.carries(typed.X)
	}
	return false
}

func (s search) passedAway(instruction ssa.Instruction) bool {
	if !s.binding.protocol.escapes.passed {
		return false
	}
	switch typed := instruction.(type) {
	case *ssa.Go:
		return s.anyCarries(operandsOf(typed.Common()))
	case *ssa.MakeClosure:
		return !deferredOnly(typed) && s.anyCarries(typed.Bindings)
	}
	return false
}

// handsOver reports whether a call passes the value as an argument. The
// receiver of a method is not handed over: calling a method uses the value.
func (s search) handsOver(common *ssa.CallCommon) bool {
	if !s.binding.protocol.escapes.passed {
		return false
	}
	arguments := common.Args
	if !common.IsInvoke() && common.Signature().Recv() != nil && len(arguments) > 0 {
		arguments = arguments[1:]
	}
	return s.anyCarries(arguments)
}

// keptOutside reports whether a store keeps the value where it outlives the
// function: a field, a package variable, an element. A local variable keeps it
// in the function, and the array of a variadic call hands it to the callee.
func (s search) keptOutside(store *ssa.Store) bool {
	if !s.carries(store.Val) {
		return false
	}
	ways := s.binding.protocol.escapes
	switch address := store.Addr.(type) {
	case *ssa.FieldAddr, *ssa.Global:
		return ways.stored
	case *ssa.IndexAddr:
		if array, ok := address.X.(*ssa.Alloc); ok && array.Comment == variadic {
			return ways.passed
		}
		return ways.stored
	}
	return false
}

func (s search) anyCarries(values []ssa.Value) bool {
	return slices.ContainsFunc(values, s.carries)
}

// carries reports whether candidate may be the value, or the address of a
// variable holding it. A candidate of unknown identity carries it only when it
// has its type: every other value a function returns or passes is not it.
func (s search) carries(candidate ssa.Value) bool {
	if candidate == nil || s.opened.value == nil {
		return false
	}
	if _, constant := candidate.(*ssa.Const); constant {
		return false
	}
	if s.holds(candidate) {
		return true
	}
	switch s.binding.compare(candidate, s.opened.value) {
	case sameYes:
		return true
	case sameUnknown:
		return sameType(candidate.Type(), s.opened.value.Type())
	}
	return false
}

// holds reports whether address is a variable the value is kept in.
func (s search) holds(address ssa.Value) bool {
	if slices.Contains(s.opened.homes, address) {
		return true
	}
	reached := pathOf(address)
	reached.steps += load
	reached.loads = true
	return reached == pathOf(s.opened.value)
}

// operandsOf is every value a call hands to the new goroutine of a go
// statement, the receiver included.
func operandsOf(common *ssa.CallCommon) []ssa.Value {
	if common.IsInvoke() {
		return append([]ssa.Value{common.Value}, common.Args...)
	}
	return common.Args
}

// deferredOnly reports whether a closure is made only to be deferred: then
// what it captures stays with the function, and §14 decides what it does.
func deferredOnly(made *ssa.MakeClosure) bool {
	referrers := made.Referrers()
	if referrers == nil || len(*referrers) == 0 {
		return false
	}
	for _, referrer := range *referrers {
		deferred, ok := referrer.(*ssa.Defer)
		if !ok || deferred.Call.Value != made {
			return false
		}
	}
	return true
}
