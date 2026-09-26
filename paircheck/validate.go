package paircheck

import (
	"errors"
	"fmt"
	"go/types"
	"maps"
	"slices"
)

// Validate checks config against the packages it names, loaded together: that
// every function and method exists, and that the types settle one slot on each
// side of every rule. The analyzer cannot check this alone: a pass sees only
// the package it analyzes and what that package imports, and a misspelled name
// matches nothing there.
func Validate(config Config, loaded []*types.Package) error {
	protocols, err := config.compile()
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
	return errors.Join(problems...)
}

// Packages lists, sorted, the import paths the rules of config name: what
// Validate needs loaded.
func Packages(config Config) ([]string, error) {
	protocols, err := config.compile()
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
	slices.Sort(paths)
	return slices.Compact(paths), nil
}
