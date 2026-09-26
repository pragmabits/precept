package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/tools/go/packages"
)

var errMode = errors.New("the modules download mode is neither mod, readonly nor vendor")

// The flags that write over the run section of a golangci-lint configuration.
const (
	flagTests        = "tests"
	flagBuildTags    = "build-tags"
	flagDownloadMode = "modules-download-mode"
)

// testVariant matches the ID go/packages gives the test variant of a package,
// "p [q.test]": the package, p, and the package whose test binary it is built
// for, q, whose test main is q.test.
var testVariant = regexp.MustCompile(`^(.*) \[(.*)\.test\]`)

// loading is how the packages are loaded, as the run section of a
// golangci-lint configuration says: with their tests or without them, under
// the build tags, and in the modules download mode.
type loading struct {
	tests        bool
	buildTags    []string
	downloadMode string
}

// overridden is l with what the command line writes over it, as in
// golangci-lint: --tests and --modules-download-mode replace the value, and
// --build-tags adds tags. The mode is checked once both have written it.
func (l loading) overridden(flags *pflag.FlagSet) (loading, error) {
	var err error
	if flags.Changed(flagTests) {
		if l.tests, err = flags.GetBool(flagTests); err != nil {
			return loading{}, err
		}
	}
	if flags.Changed(flagBuildTags) {
		added, err := flags.GetStringSlice(flagBuildTags)
		if err != nil {
			return loading{}, err
		}
		l.buildTags = append(slices.Clone(l.buildTags), added...)
	}
	if flags.Changed(flagDownloadMode) {
		if l.downloadMode, err = flags.GetString(flagDownloadMode); err != nil {
			return loading{}, err
		}
	}
	if !slices.Contains([]string{"", "mod", "readonly", "vendor"}, l.downloadMode) {
		return loading{}, fmt.Errorf("%w: %s", errMode, l.downloadMode)
	}
	return l, nil
}

// packagesConfig is what go/packages loads with: the tests, and the build
// flags golangci-lint passes go (makeBuildFlags, pkg/lint/package.go), but
// -buildvcs=false, which go/packages passes itself.
func (l loading) packagesConfig(mode packages.LoadMode) *packages.Config {
	var flags []string
	if len(l.buildTags) > 0 {
		flags = append(flags, "-tags", strings.Join(l.buildTags, " "))
	}
	if l.downloadMode != "" {
		flags = append(flags, "-mod="+l.downloadMode)
	}
	return &packages.Config{Mode: mode, Tests: l.tests, BuildFlags: flags}
}

// split separates packages loaded with their tests into plain, the packages
// as they build without their tests, and analyzed, what golangci-lint
// analyzes: the test variant of a package in place of the package, whose
// files it holds, and no test main that go test generates. A pass over the
// test main would see what testing imports, and bind rules no package of the
// project can. A test main is one a loaded test variant is built for: a main
// package of the project whose import path ends in .test is analyzed, where
// golangci-lint drops it.
func split(loaded []*packages.Package) (plain, analyzed []*packages.Package) {
	variants := make(map[string]bool)
	tested := make(map[string]bool)
	mains := make(map[string]bool)
	for _, current := range loaded {
		match := testVariant.FindStringSubmatch(current.ID)
		if match == nil {
			continue
		}
		variants[current.ID] = true
		tested[match[1]] = true
		mains[match[2]+".test"] = true
	}
	for _, current := range loaded {
		switch {
		case variants[current.ID]:
			analyzed = append(analyzed, current)
		case current.Name == "main" && mains[current.PkgPath]:
		case tested[current.PkgPath]:
			plain = append(plain, current)
		default:
			plain = append(plain, current)
			analyzed = append(analyzed, current)
		}
	}
	return plain, analyzed
}

// loadErrors joins the distinct errors of the packages and of what they
// import. An error with no position, such as an import cycle, is named by its
// package.
func loadErrors(loaded []*packages.Package) error {
	var problems []error
	seen := make(map[string]bool)
	packages.Visit(loaded, nil, func(current *packages.Package) {
		for _, problem := range current.Errors {
			found := error(problem)
			if problem.Pos == "" {
				found = fmt.Errorf("%s: %s", current.ID, problem.Msg)
			}
			if seen[found.Error()] {
				continue
			}
			seen[found.Error()] = true
			problems = append(problems, found)
		}
	})
	return errors.Join(problems...)
}
