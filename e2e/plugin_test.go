package e2e_test

import (
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
	graph := analyze(t, pluginAnalyzers(t, rules))
	var got []diagnostic
	for _, root := range graph.Roots {
		if root.Err != nil {
			t.Fatalf("%s: %v", root, root.Err)
		}
		for _, finding := range root.Diagnostics {
			at := root.Package.Fset.Position(finding.Pos)
			got = append(got, diagnostic{
				file:    relative(t, at.Filename),
				line:    at.Line,
				message: finding.Message,
			})
		}
	}
	compare(t, got)
}

// TestPluginSkipsTestMain runs a rule only the generated test main could bind.
func TestPluginSkipsTestMain(t *testing.T) {
	graph := analyze(t, pluginAnalyzers(t, testMain))
	for _, root := range graph.Roots {
		if root.Err != nil || len(root.Diagnostics) > 0 {
			t.Errorf("%s: error %v, %d diagnostics, want none", root, root.Err, len(root.Diagnostics))
		}
	}
}

// TestPluginWithoutSettings hands the plugin what golangci-lint hands it when
// paircheck is enabled with no settings.
func TestPluginWithoutSettings(t *testing.T) {
	graph := analyze(t, pluginAnalyzersOf(t, nil))
	for _, root := range graph.Roots {
		if root.Err != nil || len(root.Diagnostics) > 0 {
			t.Errorf("%s: error %v, %d diagnostics, want none", root, root.Err, len(root.Diagnostics))
		}
	}
}

func TestPluginRefuses(t *testing.T) {
	for _, current := range refusals {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			graph := analyze(t, pluginAnalyzers(t, current.file))
			failed := slices.ContainsFunc(graph.Roots, func(root *checker.Action) bool {
				return root.Err != nil && refused(root.Err.Error(), current)
			})
			if !failed {
				t.Errorf("no package failed with %q", current.want)
			}
		})
	}
}

// pluginAnalyzers builds the analyzers of the registered plugin from the rules
// in path, handed over the way golangci-lint hands them.
func pluginAnalyzers(t *testing.T, path string) []*analysis.Analyzer {
	t.Helper()
	return pluginAnalyzersOf(t, settingsOf(t, path))
}

// pluginAnalyzersOf builds the analyzers of the registered plugin from
// settings.
func pluginAnalyzersOf(t *testing.T, settings any) []*analysis.Analyzer {
	t.Helper()
	constructor, err := register.GetPlugin("paircheck")
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
func analyze(t *testing.T, analyzers []*analysis.Analyzer) *checker.Graph {
	t.Helper()
	loaded, err := packages.Load(
		&packages.Config{
			Mode:  packages.LoadAllSyntax | packages.NeedModule,
			Dir:   project,
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
