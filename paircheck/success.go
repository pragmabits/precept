package paircheck

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/ssa"
)

// constructors are the functions of the standard library documented to return
// an error that is not nil.
var constructors = []string{"errors.New", "fmt.Errorf"}

// returnsFailure reports whether ret hands back an error the path knows is not
// nil. A function whose last result is not an error has none to hand back, so
// its every return is a success.
func (s search) returnsFailure(current state, ret *ssa.Return) bool {
	results := ret.Parent().Signature.Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return false
	}
	return s.fails(current, ret.Results[last])
}

// remembered is current after instruction, which may store an error into a
// variable: a failure makes the variable hold one, and anything else stored
// where a failure was makes it no longer hold one.
func (s search) remembered(current state, instruction ssa.Instruction) state {
	store, ok := instruction.(*ssa.Store)
	if !ok || !s.binding.protocol.onSuccess || !types.Identical(store.Val.Type(), errorType) {
		return current
	}
	switch {
	case s.fails(current, store.Val):
		current.failedVariable = store.Addr
	case store.Addr == current.failedVariable:
		current.failedVariable = nil
	}
	return current
}

// fails reports whether value is an error the path knows is not nil: one built
// that way, the value a check on the path found not nil, or a load of the
// variable a failure was stored into.
func (s search) fails(current state, value ssa.Value) bool {
	if value == current.failedValue {
		return true
	}
	if variable, ok := variableRead(value); ok && variable == current.failedVariable {
		return true
	}
	return s.built(value, make(map[*ssa.Phi]bool))
}

// built reports whether value is an error that cannot be nil: a concrete value
// converted to an error, what an error constructor returned, a package-level
// error variable, or a phi of such errors. A package-level variable left nil
// would read as a failure: it is the one error taken on convention.
func (s search) built(value ssa.Value, visiting map[*ssa.Phi]bool) bool {
	switch typed := value.(type) {
	case *ssa.MakeInterface:
		return true
	case *ssa.Call:
		return s.constructs(typed)
	case *ssa.Extract:
		call, ok := typed.Tuple.(*ssa.Call)
		return ok && typed.Index == call.Call.Signature().Results().Len()-1 && s.constructs(call)
	case *ssa.UnOp:
		_, global := typed.X.(*ssa.Global)
		return typed.Op == token.MUL && global
	case *ssa.Phi:
		return s.builtOnEveryEdge(typed, visiting)
	}
	return false
}

func (s search) builtOnEveryEdge(phi *ssa.Phi, visiting map[*ssa.Phi]bool) bool {
	if visiting[phi] {
		return false
	}
	visiting[phi] = true
	return !slices.ContainsFunc(phi.Edges, func(edge ssa.Value) bool {
		return !s.built(edge, visiting)
	})
}

// constructs reports whether call returns an error that is not nil: a call of
// errors.New, of fmt.Errorf, or of a failure the configuration lists.
func (s search) constructs(call *ssa.Call) bool {
	callee := calleeOf(call.Common())
	if callee == nil {
		return false
	}
	if slices.Contains(constructors, callee.FullName()) {
		return true
	}
	return slices.ContainsFunc(s.binding.protocol.failures, func(name qualifiedName) bool {
		return name.matches(callee)
	})
}

// along is s on the successor at position of branch. Where branch finds an
// error not nil, s holds it as a failure; where it finds it nil, s no longer
// does.
func (s state) along(branch *ssa.If, position int) state {
	checked, failing, ok := nilCheck(branch.Cond)
	if !ok {
		return s
	}
	variable, read := variableRead(checked)
	switch {
	case position == failing && read:
		s.failedVariable = variable
	case position == failing:
		s.failedValue = checked
	case checked == s.failedValue:
		s.failedValue = nil
	case read && variable == s.failedVariable:
		s.failedVariable = nil
	}
	return s
}

// nilCheck is the error condition compares with nil, and the position of the
// successor taken where that error is not nil.
func nilCheck(condition ssa.Value) (ssa.Value, int, bool) {
	comparison, ok := condition.(*ssa.BinOp)
	if !ok || comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return nil, 0, false
	}
	checked := comparison.X
	switch {
	case isNil(comparison.X):
		checked = comparison.Y
	case !isNil(comparison.Y):
		return nil, 0, false
	}
	if !types.Identical(checked.Type(), errorType) {
		return nil, 0, false
	}
	if comparison.Op == token.NEQ {
		return checked, 0, true
	}
	return checked, 1, true
}

// variableRead is the variable value is loaded from, if it is a load.
func variableRead(value ssa.Value) (ssa.Value, bool) {
	load, ok := value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return nil, false
	}
	return load.X, true
}
