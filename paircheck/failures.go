package paircheck

import (
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

// constructors are the functions of the standard library documented to return
// an error that is not nil.
var constructors = []string{"errors.New", "fmt.Errorf"}

// failures is what a pass knows of the errors that are not nil: those the
// constructors of the standard library and the functions the configuration
// lists return, those the functions of the analyzed package always return, and
// those error variables hold. An error variable is taken to hold one, by
// convention, unless it is of the analyzed package and the package shows it
// may hold nil.
type failures struct {
	listed    []qualifiedName
	analyzed  *ssa.Package
	functions map[*types.Func]bool
	nilable   map[*ssa.Global]bool
}

// failuresOf is what the analyzed package tells of failures, besides those
// listed. Each kind is learned again until nothing is left: a variable can hold
// what another held, and a function can build its failure with another.
func failuresOf(program *buildssa.SSA, listed []qualifiedName) failures {
	known := failures{
		listed:    listed,
		analyzed:  program.Pkg,
		functions: make(map[*types.Func]bool),
		nilable:   make(map[*ssa.Global]bool),
	}
	assigned := assignmentsOf(program)
	for learned := true; learned; {
		learned = known.learnNilable(assigned)
	}
	for learned := true; learned; {
		learned = known.learnFunctions(program.SrcFuncs)
	}
	return known
}

// learnNilable marks each error variable of the package that may hold nil: one
// the initialization of the package never sets, or one the package sets to nil
// or to a variable that may hold nil. A variable whose address the package
// hands elsewhere may be set anywhere, and keeps the convention.
func (f failures) learnNilable(assigned assignments) bool {
	learned := false
	for _, variable := range assigned.variables {
		if f.nilable[variable] || assigned.escaped[variable] {
			continue
		}
		stores := assigned.stores[variable]
		initialized := slices.ContainsFunc(stores, func(store *ssa.Store) bool {
			return assigned.initializing(store.Parent())
		})
		if !initialized || slices.ContainsFunc(stores, f.storesNil) {
			f.nilable[variable] = true
			learned = true
		}
	}
	return learned
}

// storesNil reports whether store puts nil in its variable, or what a variable
// that may hold nil holds.
func (f failures) storesNil(store *ssa.Store) bool {
	if constant, ok := store.Val.(*ssa.Const); ok {
		return constant.IsNil()
	}
	load, ok := store.Val.(*ssa.UnOp)
	return ok && !f.holds(load)
}

// learnFunctions marks each function whose every return hands back a failure.
func (f failures) learnFunctions(functions []*ssa.Function) bool {
	learned := false
	for _, function := range functions {
		object, ok := function.Object().(*types.Func)
		if ok && !f.functions[object] && f.alwaysFails(function) {
			f.functions[object] = true
			learned = true
		}
	}
	return learned
}

// alwaysFails reports whether function has a body whose every return hands
// back an error built as a failure.
func (f failures) alwaysFails(function *ssa.Function) bool {
	results := function.Signature.Results()
	last := results.Len() - 1
	if last < 0 || !types.Identical(results.At(last).Type(), errorType) {
		return false
	}
	returned := false
	for _, block := range function.Blocks {
		ret, ok := block.Instrs[len(block.Instrs)-1].(*ssa.Return)
		if !ok {
			continue
		}
		if !f.built(ret.Results[last], make(map[*ssa.Phi]bool)) {
			return false
		}
		returned = true
	}
	return returned
}

// built reports whether value is an error that cannot be nil: a concrete value
// converted to an error, what a failure returned, an error variable holding a
// failure, or a phi of such errors.
func (f failures) built(value ssa.Value, visiting map[*ssa.Phi]bool) bool {
	switch typed := value.(type) {
	case *ssa.MakeInterface:
		return true
	case *ssa.Call:
		return f.constructs(typed)
	case *ssa.Extract:
		return f.constructsLast(typed)
	case *ssa.UnOp:
		return f.holds(typed)
	case *ssa.Phi:
		return f.builtOnEveryEdge(typed, visiting)
	}
	return false
}

// constructsLast reports whether extract is the last result of a call that
// returns an error that is not nil there.
func (f failures) constructsLast(extract *ssa.Extract) bool {
	call, ok := extract.Tuple.(*ssa.Call)
	return ok && extract.Index == call.Call.Signature().Results().Len()-1 && f.constructs(call)
}

// holds reports whether load reads an error variable holding a failure: one of
// another package, or one of the analyzed package that cannot hold nil.
func (f failures) holds(load *ssa.UnOp) bool {
	global, ok := load.X.(*ssa.Global)
	return load.Op == token.MUL && ok && (global.Pkg != f.analyzed || !f.nilable[global])
}

func (f failures) builtOnEveryEdge(phi *ssa.Phi, visiting map[*ssa.Phi]bool) bool {
	if visiting[phi] {
		return false
	}
	visiting[phi] = true
	return !slices.ContainsFunc(phi.Edges, func(edge ssa.Value) bool {
		return !f.built(edge, visiting)
	})
}

// constructs reports whether call returns an error that is not nil: a call of
// errors.New, of fmt.Errorf, of a failure the configuration lists, or of a
// function of the analyzed package that always returns one.
func (f failures) constructs(call *ssa.Call) bool {
	callee := calleeOf(call.Common())
	if callee == nil {
		return false
	}
	if slices.Contains(constructors, callee.FullName()) || f.functions[callee.Origin()] {
		return true
	}
	return slices.ContainsFunc(f.listed, func(name qualifiedName) bool {
		return name.matches(callee)
	})
}

// assignments are the error variables of a package, the stores into each, and
// the variables whose address the package hands elsewhere, where any code can
// write to them.
type assignments struct {
	initializer *ssa.Function
	variables   []*ssa.Global
	stores      map[*ssa.Global][]*ssa.Store
	escaped     map[*ssa.Global]bool
}

// assignmentsOf reads the functions of program and its initializer, which
// holds the initialization of its package-level variables.
func assignmentsOf(program *buildssa.SSA) assignments {
	assigned := assignments{
		initializer: program.Pkg.Func("init"),
		stores:      make(map[*ssa.Global][]*ssa.Store),
		escaped:     make(map[*ssa.Global]bool),
	}
	for _, member := range program.Pkg.Members {
		if variable, ok := member.(*ssa.Global); ok && holdsError(variable) {
			assigned.variables = append(assigned.variables, variable)
		}
	}
	functions := slices.Clip(program.SrcFuncs)
	if assigned.initializer != nil {
		functions = append(functions, assigned.initializer)
	}
	for _, function := range functions {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				assigned.read(program.Pkg, instruction)
			}
		}
	}
	return assigned
}

// read records instruction if it stores into an error variable of analyzed, or
// hands the address of one elsewhere: anything but a load or a store.
func (a assignments) read(analyzed *ssa.Package, instruction ssa.Instruction) {
	for _, operand := range instruction.Operands(nil) {
		variable, ok := (*operand).(*ssa.Global)
		if !ok || variable.Pkg != analyzed || !holdsError(variable) {
			continue
		}
		switch typed := instruction.(type) {
		case *ssa.Store:
			if typed.Addr == variable {
				a.stores[variable] = append(a.stores[variable], typed)
				continue
			}
		case *ssa.UnOp:
			if typed.Op == token.MUL {
				continue
			}
		}
		a.escaped[variable] = true
	}
}

// initializing reports whether function initializes the package: its
// initializer, or an init function it calls.
func (a assignments) initializing(function *ssa.Function) bool {
	return function == a.initializer ||
		function.Parent() == nil && strings.HasPrefix(function.Name(), "init#")
}

func holdsError(variable *ssa.Global) bool {
	pointer, ok := variable.Type().(*types.Pointer)
	return ok && types.Identical(pointer.Elem(), errorType)
}
