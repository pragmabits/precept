package e2e_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	// The plugin registers itself when imported, as golangci-lint imports it.
	_ "github.com/pragmabits/precept/plugin"
)

func TestPluginReports(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			graph := current.analyze(t, current.pluginAnalyzers(t, current.rules))
			current.compare(t, current.reported(t, graph))
		})
	}
}

// TestPluginSkipsTestMain runs a rule only the generated test main could bind.
func TestPluginSkipsTestMain(t *testing.T) {
	graph := paircheck.analyze(t, paircheck.pluginAnalyzers(t, testMain))
	for _, root := range graph.Roots {
		if root.Err != nil || len(root.Diagnostics) > 0 {
			t.Errorf("%s: error %v, %d diagnostics, want none", root, root.Err, len(root.Diagnostics))
		}
	}
}

// TestPluginWithoutSettings hands each plugin what golangci-lint hands it
// when its linter is enabled with no settings.
func TestPluginWithoutSettings(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			graph := current.analyze(t, current.pluginAnalyzersOf(t, nil))
			for _, root := range graph.Roots {
				if root.Err != nil || len(root.Diagnostics) > 0 {
					t.Errorf(
						"%s: error %v, %d diagnostics, want none",
						root,
						root.Err,
						len(root.Diagnostics),
					)
				}
			}
		})
	}
}

func TestPluginRefuses(t *testing.T) {
	for _, current := range paircheck.refused {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			graph := paircheck.analyze(t, paircheck.pluginAnalyzers(t, current.file))
			failed := slices.ContainsFunc(graph.Roots, func(root *checker.Action) bool {
				return root.Err != nil && refused(root.Err.Error(), current)
			})
			if !failed {
				t.Errorf("no package failed with %q", current.want)
			}
		})
	}
}

// pluginAnalyzers builds the analyzers of the linter's registered plugin from
// the rules in path, handed over the way golangci-lint hands them.
func (l linter) pluginAnalyzers(t *testing.T, path string) []*analysis.Analyzer {
	t.Helper()
	return l.pluginAnalyzersOf(t, settingsOf(t, path))
}

// pluginAnalyzersOf builds the analyzers of the linter's registered plugin
// from settings.
func (l linter) pluginAnalyzersOf(t *testing.T, settings any) []*analysis.Analyzer {
	t.Helper()
	constructor, err := register.GetPlugin(l.name)
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	linter, err := constructor(settings)
	if err != nil {
		t.Fatalf("plugin: %v", err)
	}
	analyzers, err := linter.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}
	return analyzers
}

// analyze runs analyzers over every package of the project, loaded with its
// module and its tests as golangci-lint loads it.
func (l linter) analyze(t *testing.T, analyzers []*analysis.Analyzer) *checker.Graph {
	t.Helper()
	loaded, err := packages.Load(
		&packages.Config{
			Mode:  packages.LoadAllSyntax | packages.NeedModule,
			Dir:   l.project,
			Tests: true,
		},
		"./...",
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if packages.PrintErrors(loaded) > 0 {
		t.Fatal("the project does not load")
	}
	graph, err := checker.Analyze(analyzers, analyzed(loaded), nil)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return graph
}

// reported is the diagnostics of the roots of graph that golangci-lint
// prints by default, where it places them: all but those in a file of
// generated code by the Go convention (linters.exclusions.generated: strict).
func (l linter) reported(t *testing.T, graph *checker.Graph) []diagnostic {
	t.Helper()
	var got []diagnostic
	for _, root := range graph.Roots {
		if root.Err != nil {
			t.Fatalf("%s: %v", root, root.Err)
		}
		for _, finding := range root.Diagnostics {
			if generated(root.Package, finding.Pos) {
				continue
			}
			at := placed(root.Package.Fset, finding.Pos)
			got = append(got, diagnostic{
				file:    l.relative(t, at.Filename),
				line:    at.Line,
				message: finding.Message,
			})
		}
	}
	return got
}

// generated reports whether position is in a file of loaded that follows the
// Go convention for generated code, as golangci-lint's strict mode reads it.
func generated(loaded *packages.Package, position token.Pos) bool {
	return slices.ContainsFunc(loaded.Syntax, func(file *ast.File) bool {
		return file.FileStart <= position && position <= file.FileEnd && ast.IsGenerated(file)
	})
}

// placed is where golangci-lint places pos (GetFilePositionFor,
// pkg/goanalysis/position.go): where a line directive maps it, when that is a
// Go file, and otherwise where it is.
func placed(files *token.FileSet, pos token.Pos) token.Position {
	mapped := files.PositionFor(pos, true)
	if filepath.Ext(mapped.Filename) != ".go" {
		return files.PositionFor(pos, false)
	}
	return mapped
}

// testVariant matches the ID of a test variant as golangci-lint does
// (pkg/lint/package.go): "p [q.test]", naming p.
var testVariant = regexp.MustCompile(`^(.*) \[(.*)\.test\]`)

// analyzed is what golangci-lint analyzes of packages loaded with their tests
// (pkg/lint/package.go): the test variant of a package in place of the
// package, which it holds, and no package named main whose import path ends in
// .test, its reading of a test main.
func analyzed(loaded []*packages.Package) []*packages.Package {
	variants := make(map[string]bool)
	tested := make(map[string]bool)
	for _, current := range loaded {
		if match := testVariant.FindStringSubmatch(current.ID); match != nil {
			variants[current.ID] = true
			tested[match[1]] = true
		}
	}
	var kept []*packages.Package
	for _, current := range loaded {
		testMain := current.Name == "main" && strings.HasSuffix(current.PkgPath, ".test")
		replaced := !variants[current.ID] && tested[current.PkgPath]
		if !testMain && !replaced {
			kept = append(kept, current)
		}
	}
	return kept
}

// TestPluginDefcheckRefuses hands the plugin each configuration defcheck
// refuses, which the plugin refuses before any package is analyzed.
func TestPluginDefcheckRefuses(t *testing.T) {
	constructor, err := register.GetPlugin(defcheck.name)
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	for _, current := range defcheck.refused {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			_, err := constructor(settingsOf(t, current.file))
			if err == nil || !refused(err.Error(), current) {
				t.Errorf("constructor error = %v, want %q", err, current.want)
			}
		})
	}
}
