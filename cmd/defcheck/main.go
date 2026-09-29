// Command defcheck runs the defcheck analyzer with the rules of a
// configuration file.
//
//	defcheck --config rules.yml ./...
//	defcheck -c .golangci.yml ./...
//	defcheck -v
//
// The file holds the rules, or is a golangci-lint configuration carrying them
// in its settings, native or as a module plugin. The packages load as in
// golangci-lint: with the test files, unless --tests=false, and under
// --build-tags and --modules-download-mode, over the run section of a
// golangci-lint configuration. A finding in a generated file is dropped as
// linters.exclusions.generated says, strict when it is not written.
//
// defcheck has no validate mode: every rule is checked when the analyzer is
// built, before any package loads.
package main

import (
	"io"
	"os"

	"github.com/spf13/pflag"
	"golang.org/x/tools/go/packages"

	"github.com/pragmabits/precept/defcheck"
	"github.com/pragmabits/precept/internal/driver"
)

const linterName = "defcheck"

var command = driver.Command{
	Name: linterName,
	Synopsis: `usage: defcheck -c file packages...
       defcheck -v
`,
	Dispatch: analyze,
}

func main() {
	if driver.VetTool(os.Args[1:]) {
		driver.Vet(linterName, defcheck.New)
	}
	os.Exit(command.Run(os.Args[1:], os.Stdout, os.Stderr))
}

// analyze runs the analysis of the packages the arguments that are not flags
// name.
func analyze(path string, flags *pflag.FlagSet, stdout, stderr io.Writer) (int, error) {
	config, setup, err := driver.Read[defcheck.Config](path, linterName)
	if err != nil {
		return driver.ExitFailed, err
	}
	load, err := driver.LoadingOf(setup, flags)
	if err != nil {
		return driver.ExitFailed, err
	}
	// The packages load as paircheck's do, from source: from export data, go
	// list compiles each package, and reports a compile error the type
	// checker then reports again.
	mode := packages.LoadAllSyntax
	return driver.Analyze(
		defcheck.New,
		config,
		mode,
		flags.Args(),
		load,
		setup.Generated,
		stdout,
		stderr,
	)
}
