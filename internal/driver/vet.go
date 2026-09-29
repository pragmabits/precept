package driver

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"
)

var errRelativeConfig = errors.New(
	"-config must be an absolute path: go vet runs the tool in the directory of each package",
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
// exports are those of the analyzer build makes without rules.
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
	var built *analysis.Analyzer
	var failure error
	vetted := *template
	vetted.Run = func(pass *analysis.Pass) (any, error) {
		once.Do(func() {
			built, failure = vetAnalyzer(*path, name, build)
		})
		err := failure
		var result any
		if err == nil {
			result, err = built.Run(pass)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", name, pass.Pkg.Path(), err)
			os.Exit(ExitFailed)
		}
		return result, nil
	}
	unitchecker.Main(&vetted)
}

// vetAnalyzer is the analyzer build makes of the rules of the file at path.
func vetAnalyzer[T any](
	path, name string,
	build func(T) (*analysis.Analyzer, error),
) (*analysis.Analyzer, error) {
	switch {
	case path == "":
		return nil, fmt.Errorf("-%s is required", vetConfig)
	case !filepath.IsAbs(path):
		return nil, fmt.Errorf("%w: %s", errRelativeConfig, path)
	}
	config, _, err := Read[T](path, name)
	if err != nil {
		return nil, err
	}
	return build(config)
}
