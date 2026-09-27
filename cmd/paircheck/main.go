// Command paircheck runs the paircheck analyzer with the rules of a
// configuration file, or validates the configuration against the packages it
// names.
//
//	paircheck --config rules.yml ./...
//	paircheck validate -c .golangci.yml
//	paircheck -v
//
// The file holds the rules, or is a golangci-lint configuration carrying them
// in its settings, native or as a module plugin. The packages load as in
// golangci-lint: with the test files, unless --tests=false, and under
// --build-tags and --modules-download-mode, over the run section of a
// golangci-lint configuration. A finding in a generated file is dropped as
// linters.exclusions.generated says, strict when it is not written.
package main

import (
	"io"
	"os"

	"github.com/spf13/pflag"
	"golang.org/x/tools/go/packages"

	"github.com/pragmabits/precept/internal/driver"
	"github.com/pragmabits/precept/paircheck"
)

const (
	linterName = "paircheck"
	validation = "validate"
)

var command = driver.Command{
	Name: linterName,
	Synopsis: `usage: paircheck -c file packages...
       paircheck validate -c file
       paircheck -v
`,
	Dispatch: dispatch,
}

func main() {
	os.Exit(command.Run(os.Args[1:], os.Stdout, os.Stderr))
}

// dispatch runs the analysis, or the validation when "validate" is the first
// argument that is not a flag. A bare "validate" is never a package pattern:
// it would name a standard library package that does not exist.
func dispatch(path string, flags *pflag.FlagSet, stdout, stderr io.Writer) (int, error) {
	patterns := flags.Args()
	validating := len(patterns) > 0 && patterns[0] == validation
	if validating {
		patterns = patterns[1:]
	}
	config, setup, err := driver.Read[paircheck.Config](path, linterName)
	if err != nil {
		return driver.ExitFailed, err
	}
	load, err := driver.LoadingOf(setup, flags)
	if err != nil {
		return driver.ExitFailed, err
	}
	if validating {
		return validate(config, load)
	}
	// The module of each package is what a name relative to the module is
	// resolved against.
	mode := packages.LoadAllSyntax | packages.NeedModule
	return driver.Analyze(
		paircheck.New,
		config,
		mode,
		patterns,
		load,
		setup.Generated,
		stdout,
		stderr,
	)
}
