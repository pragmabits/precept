package main

import (
	"errors"
	"io"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/pragmabits/precept/paircheck"
)

var errNoPatterns = errors.New("no packages to analyze")

func analyze(
	config paircheck.Config,
	patterns []string,
	load loading,
	stdout io.Writer,
) (int, error) {
	if len(patterns) == 0 {
		return exitFailed, errNoPatterns
	}
	analyzer, err := paircheck.New(config)
	if err != nil {
		return exitFailed, err
	}
	// The module of each package is what a name relative to the module is
	// resolved against.
	mode := packages.LoadAllSyntax | packages.NeedModule
	loaded, err := packages.Load(load.packagesConfig(mode), patterns...)
	if err != nil {
		return exitFailed, err
	}
	plain, roots := split(loaded)
	// The errors come in the order of the build: a test variant holds its
	// package and fails with it, so its errors are the tests' own only once
	// the packages load.
	if err := loadErrors(plain); err != nil {
		return exitFailed, err
	}
	if err := loadErrors(roots); err != nil {
		return exitFailed, err
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{analyzer}, roots, nil)
	if err != nil {
		return exitFailed, err
	}
	if err := rootErrors(graph); err != nil {
		return exitFailed, err
	}
	if err := graph.PrintText(stdout, -1); err != nil {
		return exitFailed, err
	}
	return outcome(graph), nil
}

// rootErrors joins the distinct errors of the analyzed packages. A rule the
// analyzer cannot bind fails every package that sees its functions, with the
// same message.
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

func outcome(graph *checker.Graph) int {
	for _, root := range graph.Roots {
		if len(root.Diagnostics) > 0 {
			return exitFindings
		}
	}
	return exitClean
}
