package paircheck

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// verdict is what a branch condition says about the error a trigger returned.
type verdict int

const (
	// verdictNone is a condition that does not read the error.
	verdictNone verdict = iota
	// verdictFailed is true when the trigger failed: failure != nil.
	verdictFailed
	// verdictSucceeded is true when the trigger succeeded: failure == nil.
	verdictSucceeded
	// verdictUnknown reads the error some other way, and leaves both branches
	// uncertain.
	verdictUnknown
)

// failure is the error a trigger call returned, and the variables it was stored
// into: a check reads it back from one of them when a closure captured it.
type failure struct {
	value ssa.Value
	homes []ssa.Value
}

func failureOf(value ssa.Value) failure {
	return failure{value: value, homes: homesOf(value)}
}

// is reports whether operand is the error: the value itself, or a load of a
// variable it was stored into.
func (f failure) is(operand ssa.Value) bool {
	if operand == f.value {
		return true
	}
	load, ok := operand.(*ssa.UnOp)
	return ok && load.Op == token.MUL && slices.Contains(f.homes, load.X)
}

// readAt is f as a check reads it at a point of the search: once another value
// displaced the error from its variables, a load of one of them is not the
// error.
func (f failure) readAt(displaced bool) failure {
	if displaced {
		f.homes = nil
	}
	return f
}

func (f failure) verdictOf(condition ssa.Value) verdict {
	if comparison, ok := condition.(*ssa.BinOp); ok && f.comparedToNil(comparison) {
		if comparison.Op == token.NEQ {
			return verdictFailed
		}
		return verdictSucceeded
	}
	if f.readBy(condition, make(map[ssa.Value]bool)) {
		return verdictUnknown
	}
	return verdictNone
}

func (f failure) comparedToNil(comparison *ssa.BinOp) bool {
	if comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return false
	}
	return f.is(comparison.X) && isNil(comparison.Y) ||
		f.is(comparison.Y) && isNil(comparison.X)
}

func isNil(value ssa.Value) bool {
	constant, ok := value.(*ssa.Const)
	return ok && constant.IsNil()
}

// readBy reports whether value is computed from the error.
func (f failure) readBy(value ssa.Value, visited map[ssa.Value]bool) bool {
	if f.is(value) {
		return true
	}
	instruction, ok := value.(ssa.Instruction)
	if !ok || visited[value] {
		return false
	}
	visited[value] = true
	for _, operand := range instruction.Operands(nil) {
		if *operand != nil && f.readBy(*operand, visited) {
			return true
		}
	}
	return false
}

// failureIndex is the result through which trigger reports failure: a final
// error. It is -1 when there is none.
func failureIndex(trigger *types.Func) int {
	results := trigger.Signature().Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return -1
	}
	return last
}
