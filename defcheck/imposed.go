package defcheck

import (
	"go/token"
	"go/types"
	"strings"
	"unicode"
	"unicode/utf8"
)

// testFunctions are the prefixes of the names of the functions go test runs,
// each with the type in package testing its one parameter points to.
var testFunctions = []struct {
	prefix    string
	parameter string
}{
	{prefix: "Test", parameter: "T"},
	{prefix: "Benchmark", parameter: "B"},
	{prefix: "Fuzz", parameter: "F"},
}

// imposed reports whether the code did not choose the name object declares:
// init, or main in package main, which the language runs; in a test file, a
// function go test runs; or a method that, with the others of its type,
// implements one of interfaces, those the package sees.
func imposed(
	files *token.FileSet,
	object types.Object,
	interfaces func() []*types.Interface,
) bool {
	function, isFunction := object.(*types.Func)
	if !isFunction {
		return false
	}
	if receiver := function.Signature().Recv(); receiver != nil {
		return implements(receiver.Type(), function.Name(), interfaces())
	}
	switch function.Name() {
	case "init":
		return true
	case "main":
		return function.Pkg().Name() == "main"
	}
	file := files.File(function.Pos())
	return file != nil && strings.HasSuffix(file.Name(), "_test.go") && runByGoTest(function)
}

// runByGoTest reports whether go test runs function, a function of a test
// file, by its name and its signature, as it reads them
// (cmd/go/internal/load/test.go): a test, a benchmark, a fuzz test, TestMain,
// or an example, which takes and returns nothing.
func runByGoTest(function *types.Func) bool {
	signature := function.Signature()
	name := function.Name()
	switch {
	case signature.TypeParams().Len() > 0:
		return false
	case name == "TestMain" && takesTesting(signature, "M"):
		return true
	case testName(name, "Example"):
		return signature.Params().Len() == 0 && signature.Results().Len() == 0
	}
	for _, test := range testFunctions {
		if testName(name, test.prefix) {
			return takesTesting(signature, test.parameter)
		}
	}
	return false
}

// testName reports whether name is prefix, or prefix and then a rune that is
// not a lower-case letter.
func testName(name, prefix string) bool {
	rest, found := strings.CutPrefix(name, prefix)
	if !found || rest == "" {
		return found
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(first)
}

// takesTesting reports whether signature takes a pointer to the type of
// package testing named parameter, and returns nothing.
func takesTesting(signature *types.Signature, parameter string) bool {
	if signature.Params().Len() != 1 || signature.Results().Len() != 0 {
		return false
	}
	pointer, isPointer := signature.Params().At(0).Type().(*types.Pointer)
	if !isPointer {
		return false
	}
	named, isNamed := types.Unalias(pointer.Elem()).(*types.Named)
	if !isNamed {
		return false
	}
	name := named.Obj()
	return name.Pkg() != nil && name.Pkg().Path() == "testing" && name.Name() == parameter
}

// implements reports whether receiver, the receiver of a method named name,
// or a pointer to it, implements one of interfaces that has a method of that
// name. The method of an interface implements nothing: its name is chosen
// where it is declared.
func implements(receiver types.Type, name string, interfaces []*types.Interface) bool {
	if pointer, isPointer := receiver.(*types.Pointer); isPointer {
		receiver = pointer.Elem()
	}
	if types.IsInterface(receiver) {
		return false
	}
	for _, candidate := range interfaces {
		if !declares(candidate, name) {
			continue
		}
		if types.Implements(receiver, candidate) ||
			types.Implements(types.NewPointer(receiver), candidate) {
			return true
		}
	}
	return false
}

// declares reports whether candidate has a method named name.
func declares(candidate *types.Interface, name string) bool {
	for index := range candidate.NumMethods() {
		if candidate.Method(index).Name() == name {
			return true
		}
	}
	return false
}

// visible is the interfaces a package sees: error, and those the package
// declares or a package it imports declares at its top level, but generic
// ones. Only the imports of the package itself count: what an import imports
// depends on how the driver loaded it, from source or from export data
// (types.Package.Imports), and drivers would disagree.
func visible(pkg *types.Package) []*types.Interface {
	builtin, _ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	interfaces := []*types.Interface{builtin}
	for _, current := range append([]*types.Package{pkg}, pkg.Imports()...) {
		scope := current.Scope()
		for _, name := range scope.Names() {
			if found, isInterface := interfaceOf(scope.Lookup(name)); isInterface {
				interfaces = append(interfaces, found)
			}
		}
	}
	return interfaces
}

// interfaceOf is the interface object names, and false when it names none, or
// a generic one.
func interfaceOf(object types.Object) (*types.Interface, bool) {
	name, isType := object.(*types.TypeName)
	if !isType {
		return nil, false
	}
	switch typed := name.Type().(type) {
	case *types.Named:
		if typed.TypeParams().Len() > 0 {
			return nil, false
		}
	case *types.Alias:
		if typed.TypeParams().Len() > 0 {
			return nil, false
		}
	}
	found, isInterface := types.Unalias(name.Type()).Underlying().(*types.Interface)
	return found, isInterface
}
