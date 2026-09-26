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
// golangci-lint configuration.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/spf13/pflag"
)

var errNoConfig = errors.New("-c, --config is required")

// The exit codes of the analysis drivers in golang.org/x/tools.
const (
	exitClean    = 0
	exitFailed   = 1
	exitFindings = 3
)

const (
	linterName = "paircheck"
	validation = "validate"
)

const usage = `usage: paircheck -c file packages...
       paircheck validate -c file
       paircheck -v

  -c, --config file   the rules: a YAML file, or a .golangci.yml carrying them
                      in its settings, native or as a module plugin
      --tests         analyze the _test.go files too (default true, or
                      run.tests of a .golangci.yml); leave them out with
                      --tests=false
      --build-tags list
                      build tags, comma-separated, added to run.build-tags of
                      a .golangci.yml
      --modules-download-mode mode
                      passed to go as -mod: mod, readonly or vendor, over
                      run.modules-download-mode of a .golangci.yml
  -v, --version       print the version and exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command: the analysis, or the validation when "validate" is the
// first argument that is not a flag. A bare "validate" is never a package
// pattern: it would name a standard library package that does not exist.
func run(arguments []string, stdout, stderr io.Writer) int {
	flags := pflag.NewFlagSet(linterName, pflag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
	}
	path := flags.StringP("config", "c", "", "the rules")
	flags.Bool(flagTests, true, "analyze the test files too")
	flags.StringSlice(flagBuildTags, nil, "build tags")
	flags.String(flagDownloadMode, "", "the modules download mode")
	showVersion := flags.BoolP("version", "v", false, "print the version and exit")
	err := flags.Parse(arguments)
	if errors.Is(err, pflag.ErrHelp) {
		return exitClean
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", linterName, err)
		flags.Usage()
		return exitFailed
	}
	if *showVersion {
		info, _ := debug.ReadBuildInfo()
		fmt.Fprintln(stdout, version(info))
		return exitClean
	}
	positional := flags.Args()
	validating := len(positional) > 0 && positional[0] == validation
	if validating {
		positional = positional[1:]
	}
	code, err := dispatch(validating, *path, flags, positional, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", linterName, err)
	}
	return code
}

func dispatch(
	validating bool,
	path string,
	flags *pflag.FlagSet,
	patterns []string,
	stdout io.Writer,
) (int, error) {
	if path == "" {
		return exitFailed, errNoConfig
	}
	config, written, err := readConfig(path)
	if err != nil {
		return exitFailed, err
	}
	load, err := written.overridden(flags)
	if err != nil {
		return exitFailed, err
	}
	if validating {
		return validate(config, load)
	}
	return analyze(config, patterns, load, stdout)
}

// version is the version of the module the binary was built from, as the go
// command recorded it: the tag it was installed at, a pseudo-version, or
// (devel) when it recorded none.
func version(info *debug.BuildInfo) string {
	if info == nil || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
