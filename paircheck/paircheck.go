// Package paircheck reports a call that opens an obligation on a value when a
// path returns from the function before a call that discharges it, for pairs
// of functions and methods the configuration declares.
package paircheck

import (
	"errors"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var errNoProgram = errors.New("the buildssa result is missing")

const linterName = "paircheck"

// New returns the analyzer enforcing the rules of config, or the error that
// makes config invalid.
func New(config Config) (*analysis.Analyzer, error) {
	protocols, err := config.compile()
	if err != nil {
		return nil, err
	}
	return &analysis.Analyzer{
		Name:     linterName,
		Doc:      "reports an obligation opened by a configured call and not discharged on every path",
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, check(pass, protocols)
		},
	}, nil
}

func check(pass *analysis.Pass, protocols []protocol) error {
	program, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return errNoProgram
	}
	protocols, err := withinModule(protocols, moduleOf(pass))
	if err != nil {
		return err
	}
	visible := visiblePackages(pass.Pkg)
	bindings := make([]binding, 0, len(protocols))
	for _, current := range protocols {
		resolved, found, err := bind(current, visible)
		if err != nil {
			return err
		}
		if found {
			bindings = append(bindings, resolved)
		}
	}
	source := indexSyntax(pass.Files)
	for _, function := range program.SrcFuncs {
		for _, current := range bindings {
			checkFunction(pass, source, current, function)
		}
	}
	return nil
}

// moduleOf is the path of the module the analyzed package belongs to, or empty
// when the driver knows of none.
func moduleOf(pass *analysis.Pass) string {
	if pass.Module == nil {
		return ""
	}
	return pass.Module.Path
}

func checkFunction(
	pass *analysis.Pass,
	source syntax,
	current binding,
	function *ssa.Function,
) {
	for _, opened := range current.obligations(function) {
		if found := (search{binding: current, opened: opened}).leaks(); found != leakNone {
			source.report(pass, current, opened, found)
		}
	}
}
