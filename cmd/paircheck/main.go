// Command paircheck runs the paircheck analyzer with the rules of a
// configuration file, or validates the configuration against the packages it
// names.
//
//	paircheck --config rules.yml ./...
//	paircheck validate -c .golangci.yml
//	paircheck -v
//
// The file holds the rules, or is a golangci-lint configuration carrying them
// in its settings, native or as a module plugin.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/types"
	"io"
	"os"
	"runtime/debug"

	"go.yaml.in/yaml/v3"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/pragmabits/precept/paircheck"
)

var (
	errNoConfig   = errors.New("-c, --config is required")
	errNoPatterns = errors.New("no packages to analyze")
	errNoSettings = errors.New("the golangci-lint configuration has no paircheck settings")
)

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
  -v, --version       print the version and exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command: the analysis, or the validation when "validate" comes
// first.
func run(arguments []string, stdout, stderr io.Writer) int {
	validating := len(arguments) > 0 && arguments[0] == validation
	if validating {
		arguments = arguments[1:]
	}
	flags := flag.NewFlagSet(linterName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
	}
	// One variable under two names is the shorthand the flag package
	// documents; either name takes one dash or two.
	var path string
	flags.StringVar(&path, "config", "", "the rules")
	flags.StringVar(&path, "c", "", "the rules (shorthand)")
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "print the version and exit")
	flags.BoolVar(&showVersion, "v", false, "print the version and exit (shorthand)")
	err := flags.Parse(arguments)
	if errors.Is(err, flag.ErrHelp) {
		return exitClean
	}
	if err != nil {
		return exitFailed
	}
	if showVersion {
		info, _ := debug.ReadBuildInfo()
		fmt.Fprintln(stdout, version(info))
		return exitClean
	}
	code, err := dispatch(validating, path, flags.Args(), stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", linterName, err)
	}
	return code
}

func dispatch(validating bool, path string, patterns []string, stdout io.Writer) (int, error) {
	if path == "" {
		return exitFailed, errNoConfig
	}
	config, err := readConfig(path)
	if err != nil {
		return exitFailed, err
	}
	if validating {
		return validate(config)
	}
	return analyze(config, patterns, stdout)
}

func analyze(config paircheck.Config, patterns []string, stdout io.Writer) (int, error) {
	if len(patterns) == 0 {
		return exitFailed, errNoPatterns
	}
	analyzer, err := paircheck.New(config)
	if err != nil {
		return exitFailed, err
	}
	loaded, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax}, patterns...)
	if err != nil {
		return exitFailed, err
	}
	if err := loadErrors(loaded); err != nil {
		return exitFailed, err
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{analyzer}, loaded, nil)
	if err != nil {
		return exitFailed, err
	}
	if err := graph.PrintText(stdout, -1); err != nil {
		return exitFailed, err
	}
	return outcome(graph), nil
}

// validate loads the packages the rules name, once, and checks the rules
// against them.
func validate(config paircheck.Config) (int, error) {
	paths, err := paircheck.Packages(config)
	if err != nil {
		return exitFailed, err
	}
	mode := packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps
	loaded, err := packages.Load(&packages.Config{Mode: mode}, paths...)
	if err != nil {
		return exitFailed, err
	}
	found := make([]*types.Package, 0, len(loaded))
	for _, current := range loaded {
		if current.Types != nil && len(current.Errors) == 0 {
			found = append(found, current.Types)
		}
	}
	if err := paircheck.Validate(config, found); err != nil {
		return exitFailed, err
	}
	return exitClean, nil
}

func outcome(graph *checker.Graph) int {
	code := exitClean
	for _, root := range graph.Roots {
		if root.Err != nil {
			return exitFailed
		}
		if len(root.Diagnostics) > 0 {
			code = exitFindings
		}
	}
	return code
}

func loadErrors(loaded []*packages.Package) error {
	var problems []error
	packages.Visit(loaded, nil, func(current *packages.Package) {
		for _, problem := range current.Errors {
			problems = append(problems, problem)
		}
	})
	return errors.Join(problems...)
}

// readConfig decodes the file the way the module plugin decodes its settings:
// through JSON, refusing a key the configuration does not have.
func readConfig(path string) (paircheck.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return paircheck.Config{}, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return paircheck.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	settings, err := settingsOf(document)
	if err != nil {
		return paircheck.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return paircheck.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var config paircheck.Config
	if err := decoder.Decode(&config); err != nil {
		return paircheck.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

// settingsOf finds the rules in a document: the document itself, or the
// paircheck settings of a golangci-lint configuration, native or as a module
// plugin.
func settingsOf(document map[string]any) (any, error) {
	linters, isGolangci := document["linters"].(map[string]any)
	if !isGolangci {
		return document, nil
	}
	settings, _ := linters["settings"].(map[string]any)
	if native, ok := settings[linterName]; ok {
		return native, nil
	}
	custom, _ := settings["custom"].(map[string]any)
	plugin, _ := custom[linterName].(map[string]any)
	if pluginSettings, ok := plugin["settings"]; ok {
		return pluginSettings, nil
	}
	return nil, errNoSettings
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
