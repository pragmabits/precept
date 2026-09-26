// Command paircheck runs the paircheck analyzer with the rules of a
// configuration file, or validates the configuration against the packages it
// names.
//
//	paircheck --config rules.yml ./...
//	paircheck validate -c .golangci.yml
//	paircheck -v
//
// The file holds the rules, or is a golangci-lint configuration carrying them
// in its settings, native or as a module plugin. The test files are analyzed
// too, as golangci-lint analyzes them: --tests decides, and without it the
// run.tests of a golangci-lint configuration.
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
	"regexp"
	"runtime/debug"
	"strconv"
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
	errRunTests   = errors.New("run.tests is neither true nor false")
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
      --tests         analyze the _test.go files too (default true, or
                      run.tests of a .golangci.yml); leave them out with
                      --tests=false
  -v, --version       print the version and exit
`

// testVariant matches the ID go/packages gives the test variant of a package,
// "p [q.test]": the package, p, and the package whose test binary it is built
// for, q, whose test main is q.test.
var testVariant = regexp.MustCompile(`^(.*) \[(.*)\.test\]`)

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
	analyzeTests := flags.Bool("tests", true, "analyze the test files too")
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
	// --tests written on the command line wins over run.tests of a
	// golangci-lint configuration, as it does in golangci-lint.
	var override *bool
	if flags.Changed("tests") {
		override = analyzeTests
	}
	code, err := dispatch(validating, *path, override, positional, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", linterName, err)
	}
	return code
}

func dispatch(
	validating bool,
	path string,
	override *bool,
	patterns []string,
	stdout io.Writer,
) (int, error) {
	if path == "" {
		return exitFailed, errNoConfig
	}
	config, tests, err := readConfig(path)
	if err != nil {
		return exitFailed, err
	}
	if validating {
		return validate(config)
	}
	if override != nil {
		tests = *override
	}
	return analyze(config, patterns, tests, stdout)
}

func analyze(
	config paircheck.Config,
	patterns []string,
	tests bool,
	stdout io.Writer,
) (int, error) {
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
	loaded, err := packages.Load(&packages.Config{Mode: mode, Tests: tests}, patterns...)
	if err != nil {
		return exitFailed, err
	}
	loaded = analyzed(loaded)
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

// validate loads the packages the rules name, once and without their tests,
// and checks the rules against them: a rule on a function declared in a
// _test.go file is refused, although the analysis applies it. A name relative
// to the module is resolved against the module of the current directory.
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

// loadErrors joins the distinct errors of the packages and of what they
// import. A file of a package with tests is loaded in the package and in its
// test variant, with the same errors in both. An error with no position, such
// as an import cycle in a test, is named by its package.
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

// analyzed is what golangci-lint analyzes of packages loaded with their tests:
// the test variant of a package in place of the package, whose files it holds,
// and no test main that go test generates. A pass over the test main would see
// what testing imports, and bind rules no package of the project can. A test
// main is one a loaded test variant is built for: a main package of the
// project whose import path ends in .test is analyzed, where golangci-lint
// drops it.
func analyzed(loaded []*packages.Package) []*packages.Package {
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
	var kept []*packages.Package
	for _, current := range loaded {
		plain := !variants[current.ID]
		testMain := plain && current.Name == "main" && mains[current.PkgPath]
		replaced := plain && tested[current.PkgPath]
		if !testMain && !replaced {
			kept = append(kept, current)
		}
	}
	return kept
}

// readConfig decodes the file the way the module plugin decodes its settings:
// through JSON, refusing a key the configuration does not have. It also says
// whether the file asks for the test files to be analyzed.
func readConfig(path string) (paircheck.Config, bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return paircheck.Config{}, false, err
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return paircheck.Config{}, false, fmt.Errorf("%s: %w", path, err)
	}
	settings, tests, err := settingsOf(document)
	if err != nil {
		return paircheck.Config{}, false, fmt.Errorf("%s: %w", path, err)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		return paircheck.Config{}, false, fmt.Errorf("%s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var config paircheck.Config
	if err := decoder.Decode(&config); err != nil {
		return paircheck.Config{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return config, tests, nil
}

// settingsOf finds what the command reads in a document: the rules, in the
// document itself or in the paircheck settings of a golangci-lint
// configuration, native or as a module plugin, and whether the test files are
// analyzed, which only a golangci-lint configuration says, in run.tests.
func settingsOf(document map[string]any) (any, bool, error) {
	linters, isGolangci := document["linters"].(map[string]any)
	if !isGolangci {
		return document, true, nil
	}
	section, _ := document["run"].(map[string]any)
	tests, err := testsOf(section["tests"])
	if err != nil {
		return nil, false, err
	}
	settings, _ := linters["settings"].(map[string]any)
	if native, ok := settings[linterName]; ok {
		return native, tests, nil
	}
	custom, _ := settings["custom"].(map[string]any)
	plugin, _ := custom[linterName].(map[string]any)
	if pluginSettings, ok := plugin["settings"]; ok {
		return pluginSettings, tests, nil
	}
	return nil, false, errNoSettings
}

// testsOf reads run.tests as golangci-lint decodes it, weakly typed: text as
// strconv.ParseBool reads it, with the empty text false, a number as whether
// it is not zero, and a key left empty as unwritten, which is true.
func testsOf(value any) (bool, error) {
	switch typed := value.(type) {
	case nil:
		return true, nil
	case bool:
		return typed, nil
	case int:
		return typed != 0, nil
	case float64:
		return typed != 0, nil
	case string:
		if typed == "" {
			return false, nil
		}
		parsed, err := strconv.ParseBool(typed)
		if err != nil {
			return false, errRunTests
		}
		return parsed, nil
	}
	return false, errRunTests
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
