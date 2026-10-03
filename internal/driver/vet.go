package driver

import (
	"errors"
	"flag"
	"fmt"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"
)

var (
	errRelativeConfig = errors.New(
		"-config must be an absolute path: go vet runs the tool in the directory of each package",
	)
	errVetBuildTags = errors.New(
		"run.build-tags cannot be applied under go vet: " +
			"pass -tags to go vet, with a configuration without the key",
	)
	errVetDirectory = errors.New(
		"run.relative-path-mode wd cannot be followed under go vet, " +
			"which runs the tool in the directory of each package",
	)
)

// vetConfig is the flag that names the rules under go vet, which hands a vet
// tool the flags the tool declares.
const vetConfig = "config"

// VetTool reports whether arguments are what go vet hands a vet tool: -V=full
// for its version, -flags for the flags it declares, or flags and then the
// file, named *.cfg, that describes one package to analyze.
func VetTool(arguments []string) bool {
	if len(arguments) == 1 && (arguments[0] == "-V=full" || arguments[0] == "-flags") {
		return true
	}
	if len(arguments) == 0 || !strings.HasSuffix(arguments[len(arguments)-1], ".cfg") {
		return false
	}
	for _, argument := range arguments[:len(arguments)-1] {
		if !strings.HasPrefix(argument, "-") {
			return false
		}
	}
	return true
}

// Vet runs as go vet's vet tool the analyzer build makes of the rules -config
// names, and does not return. The rules are read once the flags are, on the
// first package, from the file by its absolute path: go vet runs the tool in
// the directory of each package. What the analyzer requires and the facts it
// exports are those of the analyzer build makes without rules. Its findings
// are dropped and placed as the command drops and places them.
//
// An error ends the tool with status 1, the error on stderr. Returned from the
// analysis, it would go in the JSON go vet reads, with status 0: go vet keeps
// the facts of that run and reports the error once, and the next run finds the
// facts and reports nothing.
func Vet[T any](name string, build func(T) (*analysis.Analyzer, error)) {
	var empty T
	template, err := build(empty)
	if err != nil {
		log.Fatalf("%s: %v", name, err)
	}
	path := flag.String(
		vetConfig,
		"",
		"the rules, by an absolute path: a YAML file, or a .golangci.yml carrying them",
	)
	var once sync.Once
	var built vetting
	var failure error
	vetted := *template
	vetted.Run = func(pass *analysis.Pass) (any, error) {
		once.Do(func() {
			built, failure = vetAnalyzer(*path, name, build)
		})
		err := failure
		var result any
		if err == nil {
			result, err = built.run(pass)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", name, pass.Pkg.Path(), err)
			os.Exit(ExitFailed)
		}
		return result, nil
	}
	unitchecker.Main(&vetted)
}

// vetAnalyzer is what runs under go vet of the rules of the file at path: the
// analyzer build makes of them, and what drops its findings.
func vetAnalyzer[T any](
	path, name string,
	build func(T) (*analysis.Analyzer, error),
) (vetting, error) {
	switch {
	case path == "":
		return vetting{}, fmt.Errorf("-%s is required", vetConfig)
	case !filepath.IsAbs(path):
		return vetting{}, fmt.Errorf("%w: %s", errRelativeConfig, path)
	}
	config, setup, err := Read[T](path, name)
	if err != nil {
		return vetting{}, err
	}
	tests, err := vetTests(setup)
	if err != nil {
		return vetting{}, err
	}
	analyzer, err := build(config)
	if err != nil {
		return vetting{}, err
	}
	filter, err := setup.filter(name)
	if err != nil {
		return vetting{}, err
	}
	// go vet runs the tool once for each package, and no run sees the
	// findings a rule may have dropped in another.
	filter.exclusions.warnUnused = false
	return vetting{analyzer: analyzer, filter: filter, tests: tests}, nil
}

// vetTests reads the run section of setup under go vet, which builds the
// packages and runs the tool in the directory of each: whether the findings in
// the test files are kept, as run.tests says. Build tags, which would leave
// files out of what go vet built, and paths read from the working directory,
// which the tool does not know, are refused; the modules download mode
// changes nothing go vet analyzes.
func vetTests(setup Setup) (bool, error) {
	tests, err := testsOf(setup.section[flagTests])
	if err != nil {
		return false, err
	}
	tags, err := tagsOf(setup.section[flagBuildTags])
	if err != nil {
		return false, err
	}
	switch {
	case len(tags) > 0:
		return false, errVetBuildTags
	case setup.relative == relativeToDirectory:
		return false, errVetDirectory
	}
	return tests, nil
}

// vetting is what runs under go vet: the analyzer, what drops its findings,
// and whether those in the test files are kept, which go vet always analyzes.
type vetting struct {
	analyzer *analysis.Analyzer
	filter   filter
	tests    bool
}

// run runs the analyzer of v over pass, and reports the findings v keeps,
// each where the command prints it, in the order the analyzer reported them.
func (v vetting) run(pass *analysis.Pass) (any, error) {
	var diagnostics []analysis.Diagnostic
	inner := *pass
	inner.Report = func(diagnostic analysis.Diagnostic) {
		diagnostics = append(diagnostics, diagnostic)
	}
	result, err := v.analyzer.Run(&inner)
	if err != nil {
		return nil, err
	}
	var reported []analysis.Diagnostic
	var found []finding
	for _, diagnostic := range diagnostics {
		compiled := pass.Fset.PositionFor(diagnostic.Pos, false).Filename
		current := finding{
			position: placed(pass.Fset, pass.Files, diagnostic.Pos),
			end:      placed(pass.Fset, pass.Files, diagnostic.End),
			message:  diagnostic.Message,
		}
		if !v.tests && strings.HasSuffix(compiled, "_test.go") ||
			filepath.Ext(current.position.Filename) != ".go" {
			continue
		}
		reported = append(reported, diagnostic)
		found = append(found, current)
	}
	kept := v.filter.kept(found, func(err error) {
		fmt.Fprintf(os.Stderr, "%s: %v\n", v.filter.linter, err)
	})
	// kept holds findings of found in their order, and identical findings
	// alike: each is the next one of found it is the same as.
	copies := make(map[*token.File]*token.File)
	for index, current := range found {
		if len(kept) == 0 || !kept[0].same(current) {
			continue
		}
		kept = kept[1:]
		diagnostic := reported[index]
		diagnostic.Pos = printedAt(pass.Fset, copies, diagnostic.Pos, current.position)
		diagnostic.End = printedAt(pass.Fset, copies, diagnostic.End, current.end)
		pass.Report(diagnostic)
	}
	return result, nil
}

// printedAt is a position in files that unitchecker prints as placed, where
// the command places position: position itself when files maps it there, and
// otherwise the same offset in a copy of its file without its line
// directives, which printedAt adds to files once and keeps in copies.
func printedAt(
	files *token.FileSet,
	copies map[*token.File]*token.File,
	position token.Pos,
	placed token.Position,
) token.Pos {
	if !position.IsValid() || files.Position(position) == placed {
		return position
	}
	file := files.File(position)
	plain, made := copies[file]
	if !made {
		plain = files.AddFile(file.Name(), -1, file.Size())
		plain.SetLines(file.Lines())
		copies[file] = plain
	}
	return plain.Pos(file.Offset(position))
}
