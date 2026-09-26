package paircheck

import (
	"errors"
	"fmt"
	"go/types"
	"maps"
	"slices"
)

var ErrNotFailure = errors.New("a failure does not return an error as its last result")

// Validate checks config against the packages it names, loaded together: that
// every function and method exists, and that the types settle one slot on each
// side of every rule. The analyzer cannot check this alone: a pass sees only
// the package it analyzes and what that package imports, and a misspelled name
// matches nothing there. A name relative to the module is resolved against
// module, the path of the module the rules are for.
func Validate(config Config, module string, loaded []*types.Package) error {
	protocols, err := compileWithin(config, module)
	if err != nil {
		return err
	}
	visible := make(map[string]*types.Package)
	for _, root := range loaded {
		maps.Copy(visible, visiblePackages(root))
	}
	var problems []error
	for _, current := range protocols {
		if _, err := resolve(current, visible, true); err != nil {
			problems = append(problems, fmt.Errorf("rule %q: %w", current.id, err))
		}
	}
	failures, err := failuresWithin(config, module)
	if err != nil {
		return err
	}
	for _, name := range failures {
		if err := checkFailure(visible, name); err != nil {
			problems = append(problems, fmt.Errorf("failures: %s: %w", name, err))
		}
	}
	return errors.Join(problems...)
}

// Packages lists, sorted, the import paths the rules of config name, with the
// relative names resolved against module: what Validate needs loaded.
func Packages(config Config, module string) ([]string, error) {
	protocols, err := compileWithin(config, module)
	if err != nil {
		return nil, err
	}
	failures, err := failuresWithin(config, module)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, current := range protocols {
		paths = append(paths, current.trigger.name.path)
		for _, satisfier := range current.satisfiers {
			if !satisfier.called {
				paths = append(paths, satisfier.name.path)
			}
		}
	}
	for _, name := range failures {
		paths = append(paths, name.path)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

// compileWithin compiles config and resolves its relative names against module.
func compileWithin(config Config, module string) ([]protocol, error) {
	protocols, err := config.compile()
	if err != nil {
		return nil, err
	}
	return withinModule(protocols, module)
}

// failuresWithin is the failures of config resolved against module.
func failuresWithin(config Config, module string) ([]qualifiedName, error) {
	failures, err := config.compileFailures()
	if err != nil {
		return nil, err
	}
	resolved, err := namesWithin(failures, module)
	if err != nil {
		return nil, fmt.Errorf("failures: %w", err)
	}
	return resolved, nil
}

// checkFailure refuses a failure that is not among the visible functions, or
// that returns no error through which it could fail.
func checkFailure(visible map[string]*types.Package, name qualifiedName) error {
	function := lookupFunc(visible, name)
	if function == nil {
		return ErrUnknownFunction
	}
	if failureIndex(function) < 0 {
		return ErrNotFailure
	}
	return nil
}
