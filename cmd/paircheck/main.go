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
	"fmt"
	"go/types"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/modfile"
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
	code, err := dispatch(validating, *path, positional, stdout)
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
	// The module of each package is what a name relative to the module is
	// resolved against.
	mode := packages.LoadAllSyntax | packages.NeedModule
	loaded, err := packages.Load(&packages.Config{Mode: mode}, patterns...)
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
	if err := rootErrors(graph); err != nil {
		return exitFailed, err
	}
	if err := graph.PrintText(stdout, -1); err != nil {
		return exitFailed, err
	}
	return outcome(graph), nil
}

// validate loads the packages the rules name, once, and checks the rules
// against them. A name relative to the module is resolved against the module
// of the current directory.
func validate(config paircheck.Config) (int, error) {
	module, err := currentModule()
	if err != nil {
		return exitFailed, err
	}
	paths, err := paircheck.Packages(config, module)
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
	if err := paircheck.Validate(config, module, found); err != nil {
		return exitFailed, err
	}
	return exitClean, nil
}

// currentModule is the path of the module the current directory belongs to,
// as the go command finds its go.mod, or empty outside a module.
func currentModule() (string, error) {
	output, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD: %w", err)
	}
	file := strings.TrimSpace(string(output))
	if file == "" || file == os.DevNull {
		return "", nil
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return modfile.ModulePath(content), nil
}

// rootErrors joins the distinct errors of the analyzed packages. A rule the
// analyzer cannot bind fails every package that sees its functions, with the
// same message.
func rootErrors(graph *checker.Graph) error {
	var problems []error
	seen := make(map[string]bool)
	for _, root := range graph.Roots {
		if root.Err == nil || seen[root.Err.Error()] {
			continue
		}
		seen[root.Err.Error()] = true
		problems = append(problems, root.Err)
	}
	return errors.Join(problems...)
}

func outcome(graph *checker.Graph) int {
	for _, root := range graph.Roots {
		if len(root.Diagnostics) > 0 {
			return exitFindings
		}
	}
	return exitClean
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
