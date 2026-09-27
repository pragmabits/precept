// Package driver runs an analyzer of this module as its command does: over
// the packages loaded as golangci-lint loads them, with the configuration read
// from a file of the command's own or from a golangci-lint configuration, and
// with the diagnostics printed one per line.
package driver

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

var errNoPatterns = errors.New("no packages to analyze")

// The exit codes of the analysis drivers in golang.org/x/tools.
const (
	ExitClean    = 0
	ExitFailed   = 1
	ExitFindings = 3
)

// Analyze builds the analyzer from config, runs it over the packages patterns
// name, loaded in mode as load says, and prints to stdout its diagnostics as
// golangci-lint prints them, where it places them and but those in the files
// generated drops, and to stderr a warning golangci-lint gives. It returns
// ExitFindings when it prints one, ExitClean when it prints none, and
// ExitFailed with the error that stopped it.
func Analyze[T any](
	build func(T) (*analysis.Analyzer, error),
	config T,
	mode packages.LoadMode,
	patterns []string,
	load Loading,
	generated Generated,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	if len(patterns) == 0 {
		return ExitFailed, errNoPatterns
	}
	analyzer, err := build(config)
	if err != nil {
		return ExitFailed, err
	}
	loaded, err := packages.Load(load.Config(mode), patterns...)
	if err != nil {
		return ExitFailed, err
	}
	plain, roots := split(loaded)
	// The errors come in the order of the build: a test variant holds its
	// package and fails with it, so its errors are the tests' own only once
	// the packages load.
	if err := loadErrors(plain); err != nil {
		return ExitFailed, err
	}
	if err := loadErrors(roots); err != nil {
		return ExitFailed, err
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{analyzer}, roots, nil)
	if err != nil {
		return ExitFailed, err
	}
	if err := rootErrors(graph); err != nil {
		return ExitFailed, err
	}
	found := report(graph, generated, func(err error) {
		fmt.Fprintf(stderr, "%s: %v: no finding in a generated file is dropped\n", analyzer.Name, err)
	})
	if err := write(stdout, found); err != nil {
		return ExitFailed, err
	}
	if len(found) > 0 {
		return ExitFindings, nil
	}
	return ExitClean, nil
}

// rootErrors joins the distinct errors of the analyzed packages. An analyzer
// may fail every package alike, as paircheck fails every package that sees
// the functions of a rule it cannot bind, with the same message.
func rootErrors(graph *checker.Graph) error {
	var problems []error
	seen := make(map[string]bool)
	for _, root := range graph.Roots {
		if root.Err == nil || seen[root.Err.Error()] {
			continue
		}
		seen[root.Err.Error()] = true
		problems = append(problems, root.Err)
	}
	return errors.Join(problems...)
}
