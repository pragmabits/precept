// Package paircheck reports a call that opens an obligation on a value when a
// path returns from the function before a call that discharges it, for pairs
// of functions and methods the configuration declares.
package paircheck

import (
	"errors"
	"go/types"

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
		Name:      linterName,
		Doc:       "reports an obligation opened by a configured call and not discharged on every path",
		Requires:  []*analysis.Analyzer{buildssa.Analyzer},
		FactTypes: []analysis.Fact{new(summary)},
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
	if !developed(pass) {
		protocols = absolute(protocols)
	}
	protocols, err := withinModule(protocols, moduleOf(pass))
	if err != nil {
		return err
	}
	bindings, err := bindAll(protocols, visiblePackages(pass.Pkg), program)
	if err != nil {
		return err
	}
	learnSummaries(pass, bindings, program.SrcFuncs)
	source := indexSyntax(pass.Files)
	for _, function := range program.SrcFuncs {
		for _, current := range bindings {
			checkFunction(pass, source, current, function)
		}
	}
	return nil
}

// bindAll binds the protocols the pass can resolve, and gives those that are
// on-success what the package tells of failures, learned once for all.
func bindAll(
	protocols []protocol,
	visible map[string]*types.Package,
	program *buildssa.SSA,
) ([]binding, error) {
	var expanded []protocol
	for _, current := range protocols {
		applied, err := implemented(current, visible)
		if err != nil {
			return nil, err
		}
		expanded = append(append(expanded, current), applied...)
	}
	bindings := make([]binding, 0, len(expanded))
	var known *failures
	for _, current := range expanded {
		resolved, found, err := bind(current, visible)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		resolved.stored = make(map[ssa.Value][]*ssa.MakeClosure)
		if current.onSuccess {
			if known == nil {
				learned := failuresOf(program, current.failures)
				known = &learned
			}
			resolved.failures = *known
		}
		bindings = append(bindings, resolved)
	}
	return bindings, nil
}

// developed reports whether pass analyzes a package of a module being
// developed, the one a name relative to the module refers to: a module with no
// version, as the main module and those of a workspace are. A package of a
// dependency, or of the standard library, belongs to none of them.
func developed(pass *analysis.Pass) bool {
	return pass.Module != nil && pass.Module.Path != "" && pass.Module.Version == ""
}

// absolute is protocols without those that name anything relative to the
// module, and without the failures named so, for a package outside the module
// the names are relative to.
func absolute(protocols []protocol) []protocol {
	kept := make([]protocol, 0, len(protocols))
	for _, current := range protocols {
		if !current.relative() {
			kept = append(kept, current.absolute())
		}
	}
	return kept
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
	obligations := current.obligations(function)
	if len(obligations) == 0 {
		return
	}
	var returned map[ssa.Value]int
	var closures map[*ssa.Defer]int
	if current.protocol.onSuccess {
		returned = returnedErrors(function)
		closures = deferredClosures(current, function)
	}
	for _, opened := range obligations {
		found := (search{
			binding:  current,
			opened:   opened,
			returned: returned,
			closures: closures,
		}).leaks()
		if found != leakNone {
			source.report(pass, current, opened, found)
		}
	}
}
