// Package defcheck reports a declaration whose name a configured pattern
// forbids for its kind of declaration, at the identifier that declares it.
package defcheck

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
)

const linterName = "defcheck"

// New returns the analyzer enforcing the rules of config, or the error that
// makes config invalid.
func New(config Config) (*analysis.Analyzer, error) {
	prohibitions, err := config.compile()
	if err != nil {
		return nil, err
	}
	return &analysis.Analyzer{
		Name: linterName,
		Doc:  "reports a declaration whose name a configured pattern forbids for its kind",
		Run: func(pass *analysis.Pass) (any, error) {
			check(pass, prohibitions)
			return nil, nil
		},
	}, nil
}

// check reports each declaration of the package for each rule that forbids
// it, in the order of the source, and at one declaration in the order of the
// rules.
func check(pass *analysis.Pass, prohibitions []prohibition) {
	for _, current := range declarations(pass.TypesInfo) {
		kind, found := kindOf(current.object)
		if !found {
			continue
		}
		for _, rule := range prohibitions {
			if rule.forbids(current.name, kind) {
				pass.Report(analysis.Diagnostic{
					Pos:     current.position,
					Message: rule.diagnose(current.name, kind),
				})
			}
		}
	}
}

// declaration is a name the package declares, where it declares it, and the
// object it defines.
type declaration struct {
	name     string
	position token.Pos
	object   types.Object
}

// declarations is every name the package declares, by position: each
// identifier Defs gives an object, but _, and the variable of each type
// switch, which Defs leaves without one. Its objects are in Implicits, one per
// clause, all at the variable, so the variable is one declaration.
func declarations(info *types.Info) []declaration {
	var found []declaration
	for identifier, object := range info.Defs {
		if object != nil && identifier.Name != "_" {
			found = append(found, declaration{
				name:     identifier.Name,
				position: identifier.Pos(),
				object:   object,
			})
		}
	}
	switches := make(map[token.Pos]bool)
	for node, object := range info.Implicits {
		if _, isClause := node.(*ast.CaseClause); !isClause || switches[object.Pos()] {
			continue
		}
		switches[object.Pos()] = true
		found = append(found, declaration{
			name:     object.Name(),
			position: object.Pos(),
			object:   object,
		})
	}
	slices.SortFunc(found, func(first, second declaration) int {
		return cmp.Compare(first.position, second.position)
	})
	return found
}
