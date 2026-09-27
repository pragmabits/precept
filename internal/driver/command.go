package driver

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/pflag"
)

var errNoConfig = errors.New("-c, --config is required")

// flagsUsage describes the flags every command takes, as its usage lists them
// under its synopsis.
const flagsUsage = `  -c, --config file   the rules: a YAML file, or a .golangci.yml carrying them
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

// Command is a command of this module: its name, the synopsis its usage
// opens with, and what it does once its flags are parsed, with the file -c
// names. Every command takes the same flags.
type Command struct {
	Name     string
	Synopsis string
	Dispatch func(path string, flags *pflag.FlagSet, stdout, stderr io.Writer) (int, error)
}

// Run is the command over arguments, whose flags are GNU style: before,
// between or after the other arguments, which Dispatch reads in flags.Args.
// -h prints the usage and -v the version, and an error goes to stderr after
// the command's name.
func (c Command) Run(arguments []string, stdout, stderr io.Writer) int {
	flags := pflag.NewFlagSet(c.Name, pflag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), c.Synopsis+"\n"+flagsUsage)
	}
	path := flags.StringP("config", "c", "", "the rules")
	addFlags(flags)
	showVersion := flags.BoolP("version", "v", false, "print the version and exit")
	err := flags.Parse(arguments)
	if errors.Is(err, pflag.ErrHelp) {
		return ExitClean
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", c.Name, err)
		flags.Usage()
		return ExitFailed
	}
	if *showVersion {
		info, _ := debug.ReadBuildInfo()
		fmt.Fprintln(stdout, Version(info))
		return ExitClean
	}
	code, err := c.dispatch(*path, flags, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", c.Name, err)
	}
	return code
}

func (c Command) dispatch(
	path string,
	flags *pflag.FlagSet,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	if path == "" {
		return ExitFailed, errNoConfig
	}
	return c.Dispatch(path, flags, stdout, stderr)
}

// Version is the version of the module the binary was built from, as the go
// command recorded it: the tag it was installed at, a pseudo-version, or
// (devel) when it recorded none.
func Version(info *debug.BuildInfo) string {
	if info == nil || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
