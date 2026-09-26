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

// loadingOf is the load that the run section of a golangci-lint configuration
// and the command line ask for, as golangci-lint reads them. A flag written on
// the command line replaces its key, which is then not read from the file, as
// viper takes a changed flag first; --build-tags adds to run.build-tags, which
// is read either way; and a key neither writes keeps its default. The mode is
// checked once it is settled. The command's own file has no run section.
func loadingOf(section map[string]any, flags *pflag.FlagSet) (loading, error) {
	var load loading
	var err error
	if flags.Changed(flagTests) {
		load.tests, err = flags.GetBool(flagTests)
	} else {
		load.tests, err = testsOf(section[flagTests])
	}
	if err != nil {
		return loading{}, err
	}
	if load.buildTags, err = tagsOf(section[flagBuildTags]); err != nil {
		return loading{}, err
	}
	if flags.Changed(flagBuildTags) {
		added, err := flags.GetStringSlice(flagBuildTags)
		if err != nil {
			return loading{}, err
		}
		load.buildTags = append(load.buildTags, added...)
	}
	if flags.Changed(flagDownloadMode) {
		load.downloadMode, err = flags.GetString(flagDownloadMode)
	} else {
		load.downloadMode, err = modeOf(section[flagDownloadMode])
	}
	if err != nil {
		return loading{}, err
	}
	if !slices.Contains([]string{"", "mod", "readonly", "vendor"}, load.downloadMode) {
		return loading{}, fmt.Errorf("%w: %s", errMode, load.downloadMode)
	}
	return load, nil
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
