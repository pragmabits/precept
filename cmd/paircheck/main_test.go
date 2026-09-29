package main

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/pragmabits/precept/internal/driver"
)

const leak = "leak.go:6:2: [resource] Open requires Close on r before function exit"

func TestAnalyzeReports(t *testing.T) {
	for _, config := range []string{"rules.yml", "golangci-native.yml", "golangci-plugin.yml"} {
		t.Run(config, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("--config", config, "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if !strings.Contains(stdout, leak) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
			}
		})
	}
}

func TestConfigSpellings(t *testing.T) {
	spellings := [][]string{
		{"-c", "rules.yml", "./..."},
		{"--config", "rules.yml", "./..."},
		{"--config=rules.yml", "./..."},
		{"./...", "-c", "rules.yml"},
		{"./leak", "--config", "rules.yml", "./resource"},
		{"-c", "rules.yml", "--", "./..."},
	}
	for _, spelling := range spellings {
		t.Run(strings.Join(spelling, " "), func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute(spelling...)
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if !strings.Contains(stdout, leak) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
			}
		})
	}
}

func TestUnknownFlag(t *testing.T) {
	code, _, stderr := execute("--verbose", "./...")
	if code != driver.ExitFailed {
		t.Errorf("exit = %d, want %d", code, driver.ExitFailed)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("stderr = %q, want it to name --verbose", stderr)
	}
}

func TestValidateTakesTheShortFlag(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	if code, _, stderr := execute("validate", "-c", "rules.yml"); code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

func TestHelp(t *testing.T) {
	code, _, stderr := execute("-h")
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d", code, driver.ExitClean)
	}
	for _, flag := range []string{
		"-c, --config",
		"--tests",
		"--build-tags",
		"--modules-download-mode",
		"-v, --version",
	} {
		if !strings.Contains(stderr, flag) {
			t.Errorf("usage = %q, want it to name %s", stderr, flag)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	for _, spelling := range []string{"-v", "--version"} {
		t.Run(spelling, func(t *testing.T) {
			code, stdout, stderr := execute(spelling)
			if code != driver.ExitClean {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
			}
			info, _ := debug.ReadBuildInfo()
			if want := driver.Version(info) + "\n"; stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		build string
		info  *debug.BuildInfo
		want  string
	}{
		{build: "module version", info: built("v0.1.0+dirty"), want: "v0.1.0+dirty"},
		{build: "no module version", info: built(""), want: "(devel)"},
		{build: "no build information", info: nil, want: "(devel)"},
	}
	for _, test := range tests {
		t.Run(test.build, func(t *testing.T) {
			if got := driver.Version(test.info); got != test.want {
				t.Errorf("version = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnalyzeRefusesInvalidConfig(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: resource
    trigger: (*example.com/project/resource.Resource).Open
    satisfiers: [(*example.com/project/resource.Resource).Close]
    transfer: true
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d", code, driver.ExitFailed)
	}
	if !strings.Contains(stderr, "transfer") {
		t.Errorf("stderr = %q, want it to name transfer", stderr)
	}
}

func TestAnalyzeRefusesUnboundRule(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: swap
    trigger: (*example.com/project/resource.Cache).Swap
    satisfiers: [(*example.com/project/resource.Cache).Put]
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	want := `rule "swap": more than one slot`
	if strings.Count(stderr, want) != 1 {
		t.Errorf("stderr = %q, want %q once", stderr, want)
	}
}

func TestAnalyzeReportsTheErrorOfADependency(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: swap
    trigger: (*example.com/project/resource.Cache).Swap
    satisfiers: [(*example.com/project/resource.Cache).Put]
`)
	code, _, stderr := execute("--config", config, "./leak")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	want := `rule "swap": more than one slot`
	if strings.Count(stderr, want) != 1 || strings.Contains(stderr, "failed prerequisites") {
		t.Errorf("stderr = %q, want %q once, and no failed prerequisites", stderr, want)
	}
}

func TestAnalyzeLeavesRelativeNamesOutOfDependencies(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "dependent"))
	config := write(t, `rules:
  - id: store
    trigger: (*./store.Store).Begin
    satisfiers: [(*./store.Store).End]
`)
	code, stdout, stderr := execute("--config", config, "./...")
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stdout: %s; stderr: %s", code, driver.ExitClean, stdout, stderr)
	}
}

func TestAnalyzePrintsLoadErrorsOnce(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "broken"))
	code, _, stderr := execute("--config", write(t, "rules: []\n"), "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	want := `value.go:5:27: cannot use "value"`
	if strings.Count(stderr, want) != 1 {
		t.Errorf("stderr = %q, want %q once", stderr, want)
	}
}

func TestAnalyzeNamesThePackageOfAnErrorWithoutPosition(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "cycles"))
	code, _, stderr := execute("--config", write(t, "rules: []\n"), "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	for _, want := range []string{
		"example.com/cycles/first [example.com/cycles/first.test]: import cycle not allowed in test",
		"example.com/cycles/third [example.com/cycles/third.test]: import cycle not allowed in test",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

func TestAnalyzeReportsACycleOutsideTheTestsAsWithoutThem(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "circular"))
	config := write(t, "rules: []\n")
	code, _, without := execute("--config", config, "--tests=false", "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, without)
	}
	code, _, with := execute("--config", config, "./...")
	if code != driver.ExitFailed || with != without {
		t.Errorf("exit = %d, stderr = %q, want %d and %q", code, with, driver.ExitFailed, without)
	}
}

func TestAnalyzeReadsKeysInAnyCase(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `Linters:
  Settings:
    PairCheck:
      rules:
        - id: resource
          trigger: (*example.com/project/resource.Resource).Open
          satisfiers: [(*example.com/project/resource.Resource).Close]
`)
	code, stdout, stderr := execute("--config", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, leak) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
	}
}

func TestAnalyzeRefusesKeysThatDifferInCase(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `run:
  tests: true
  Tests: false
linters:
  settings:
    paircheck:
      rules: []
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	if !strings.Contains(stderr, "differ only in case") {
		t.Errorf("stderr = %q, want it to say the keys differ only in case", stderr)
	}
}

func TestAnalyzeKeepsAMainNamedLikeATestMain(t *testing.T) {
	rules := `rules:
  - id: file
    trigger: os.Open
    satisfiers: [(*os.File).Close]
`
	want := "main.go:8:15: [file] Open requires Close on file before function exit"
	for _, spelling := range []string{"--tests", "--tests=false"} {
		t.Run(spelling, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "lookalike"))
			code, stdout, stderr := execute("--config", write(t, rules), spelling, "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if !strings.Contains(stdout, want) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, want)
			}
		})
	}
}

func TestAnalyzeFollowsModulesDownloadMode(t *testing.T) {
	tests := []struct {
		name   string
		run    string
		flags  []string
		failed bool
	}{
		{name: "unwritten", run: "", flags: nil, failed: false},
		{name: "vendor in the file", run: "vendor", flags: nil, failed: true},
		{
			name:   "vendor on the command line",
			run:    "",
			flags:  []string{"--modules-download-mode=vendor"},
			failed: true,
		},
		{
			name:   "the flag wins",
			run:    "vendor",
			flags:  []string{"--modules-download-mode", "mod"},
			failed: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "replaced"))
			config := write(t, "run:\n  modules-download-mode: \""+test.run+"\"\n"+
				"linters:\n  settings:\n    paircheck:\n      rules: []\n")
			arguments := append([]string{"--config", config}, test.flags...)
			code, _, stderr := execute(append(arguments, "./...")...)
			vendoring := strings.Contains(stderr, "inconsistent vendoring")
			if code == driver.ExitFailed != test.failed || vendoring != test.failed {
				t.Errorf("exit = %d, stderr = %q, want a vendoring failure: %t", code, stderr, test.failed)
			}
		})
	}
}

func TestAnalyzeRefusesModulesDownloadMode(t *testing.T) {
	tests := []struct {
		name  string
		run   string
		flags []string
	}{
		{name: "in the file", run: "download", flags: nil},
		{name: "on the command line", run: "", flags: []string{"--modules-download-mode=download"}},
		{name: "a number in the file", run: "1", flags: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, "run:\n  modules-download-mode: "+cmp.Or(test.run, `""`)+"\n"+
				"linters:\n  settings:\n    paircheck:\n      rules: []\n")
			arguments := append([]string{"--config", config}, test.flags...)
			code, _, stderr := execute(append(arguments, "./...")...)
			if code != driver.ExitFailed ||
				!strings.Contains(stderr, "neither mod, readonly nor vendor") {
				t.Errorf(
					"exit = %d, stderr = %q, want %d and the mode refused",
					code,
					stderr,
					driver.ExitFailed,
				)
			}
		})
	}
}

func TestValidateFollowsBuildTags(t *testing.T) {
	tests := []struct {
		name  string
		run   string
		flags []string
		code  int
	}{
		{name: "no tag", run: "[]", flags: nil, code: driver.ExitFailed},
		{name: "in the file", run: "[precept]", flags: nil, code: driver.ExitClean},
		{
			name:  "on the command line",
			run:   "[]",
			flags: []string{"--build-tags", "precept"},
			code:  driver.ExitClean,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "tagged"))
			config := write(t, "run:\n  build-tags: "+test.run+"\n"+
				"linters:\n  settings:\n    paircheck:\n      rules:\n"+
				"        - id: handle\n          trigger: ./handle.Open\n"+
				"          satisfiers: [(*./handle.Handle).Close]\n")
			arguments := append([]string{"validate", "--config", config}, test.flags...)
			if code, _, stderr := execute(arguments...); code != test.code {
				t.Errorf("exit = %d, want %d; stderr: %s", code, test.code, stderr)
			}
		})
	}
}

func TestAnalyzeDoesNotReadAKeyAWrittenFlagReplaces(t *testing.T) {
	tests := []struct {
		name string
		run  string
		flag string
	}{
		{name: "tests", run: "tests: sometimes", flag: "--tests=false"},
		{name: "mode", run: "modules-download-mode: [vendor]", flag: "--modules-download-mode=mod"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, "run:\n  "+test.run+"\n"+`linters:
  settings:
    paircheck:
      rules:
        - id: resource
          trigger: (*example.com/project/resource.Resource).Open
          satisfiers: [(*example.com/project/resource.Resource).Close]
`)
			code, stdout, stderr := execute("--config", config, test.flag, "./...")
			if code != driver.ExitFindings || !strings.Contains(stdout, leak) {
				t.Errorf("exit = %d, stdout = %q, want %d and %q; stderr: %s",
					code, stdout, driver.ExitFindings, leak, stderr)
			}
		})
	}
}

func TestAnalyzeRefusesRunTests(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `run:
  tests: sometimes
linters:
  settings:
    paircheck:
      rules: []
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFailed, stderr)
	}
	if !strings.Contains(stderr, "run.tests") {
		t.Errorf("stderr = %q, want it to name run.tests", stderr)
	}
}

// fileRule is the rule of testdata/generated, which leaks a file in each of
// its files, written as the paircheck settings of a golangci-lint
// configuration.
const fileRule = `  settings:
    paircheck:
      rules:
        - id: file
          trigger: os.Open
          satisfiers: [(*os.File).Close]
`

// TestAnalyzeDropsFindingsInGeneratedFiles reads linters.exclusions.generated
// as golangci-lint does, weakly typed and under a key of any case, and drops
// the findings in the files it takes for generated. A case with no key writes
// generated.
func TestAnalyzeDropsFindingsInGeneratedFiles(t *testing.T) {
	strict := []string{"after.go", "lax.go", "plain.go"}
	lax := []string{"plain.go"}
	every := []string{"after.go", "lax.go", "plain.go", "strict.go"}
	tests := []struct {
		name     string
		key      string
		value    string
		reported []string
	}{
		{name: "strict", value: "strict", reported: strict},
		{name: "empty text", value: `""`, reported: strict},
		{name: "null", value: "null", reported: strict},
		{name: "empty map", value: "{}", reported: strict},
		{name: "lax", value: "lax", reported: lax},
		{name: "unknown text", value: "sometimes", reported: lax},
		{name: "boolean", value: "false", reported: lax},
		{name: "number", value: "2.5", reported: lax},
		{name: "disable", value: "disable", reported: every},
		{name: "capitalized key", key: "Generated", value: "lax", reported: lax},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "generated"))
			key := cmp.Or(test.key, "generated")
			config := write(t, "linters:\n  exclusions:\n    "+key+": "+test.value+"\n"+fileRule)
			code, stdout, stderr := execute("--config", config, "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if got := reported(stdout); !slices.Equal(got, test.reported) {
				t.Errorf("findings in %v, want in %v", got, test.reported)
			}
		})
	}
	unwritten := []struct {
		file    string
		content string
	}{
		{
			file:    "own file",
			content: "rules:\n  - id: file\n    trigger: os.Open\n    satisfiers: [(*os.File).Close]\n",
		},
		{
			file:    "golangci-lint file",
			content: "linters:\n" + fileRule,
		},
	}
	for _, test := range unwritten {
		t.Run(test.file, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "generated"))
			code, stdout, stderr := execute("--config", write(t, test.content), "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if got := reported(stdout); !slices.Equal(got, strict) {
				t.Errorf("findings in %v, want in %v", got, strict)
			}
		})
	}
}

// TestAnalyzePlacesACgoFindingInTheGoFile reports a leak in a file that
// imports C where golangci-lint places it: in that file, not in the one cgo
// rewrites in the build cache, which is generated.
func TestAnalyzePlacesACgoFindingInTheGoFile(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "cgo"))
	config := write(t, "linters:\n"+fileRule)
	code, stdout, stderr := execute("--config", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	want := "main.go:12:15: [file] Open requires Close on file before function exit"
	if !strings.HasSuffix(strings.TrimSpace(stdout), want) {
		t.Errorf("stdout = %q, want it to end with %q", stdout, want)
	}
}

func TestAnalyzeRefusesGeneratedMode(t *testing.T) {
	for _, value := range []string{"[lax]", "[]", "{mode: lax}"} {
		t.Run(value, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "generated"))
			config := write(t, "linters:\n  exclusions:\n    generated: "+value+"\n"+fileRule)
			code, stdout, stderr := execute("--config", config, "./...")
			if code != driver.ExitFailed || stdout != "" ||
				!strings.Contains(stderr, "linters.exclusions.generated") {
				t.Errorf(
					"exit = %d, stdout = %q, stderr = %q, want %d and the mode refused",
					code,
					stdout,
					stderr,
					driver.ExitFailed,
				)
			}
			code, _, stderr = execute("validate", "--config", config)
			if code != driver.ExitFailed || !strings.Contains(stderr, "linters.exclusions.generated") {
				t.Errorf("validate: exit = %d, stderr = %q, want %d and the mode refused",
					code, stderr, driver.ExitFailed)
			}
		})
	}
}

// TestRefusesNativeAndPluginSettings writes the rules both as native settings
// and as module plugin settings: golangci-lint would apply the plugin's, and
// the file does not say which the command applies.
func TestRefusesNativeAndPluginSettings(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `linters:
  settings:
    paircheck:
      rules: []
    custom:
      paircheck:
        type: module
        settings:
          rules: []
`)
	for _, arguments := range [][]string{{"-c", config, "./..."}, {"validate", "-c", config}} {
		t.Run(arguments[0], func(t *testing.T) {
			code, stdout, stderr := execute(arguments...)
			want := "both native and module plugin paircheck settings"
			if code != driver.ExitFailed || stdout != "" || !strings.Contains(stderr, want) {
				t.Errorf(
					"exit = %d, stdout = %q, stderr = %q, want %d and %q",
					code,
					stdout,
					stderr,
					driver.ExitFailed,
					want,
				)
			}
		})
	}
}

func TestRequiresConfig(t *testing.T) {
	if code, _, _ := execute("./..."); code != driver.ExitFailed {
		t.Errorf("exit = %d, want %d", code, driver.ExitFailed)
	}
}

func TestValidateAcceptsImplementations(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: opener
    trigger: (./resource.Opener).Open
    satisfiers: [(./resource.Opener).Close]
    implementations: [./resource.Resource]
`)
	code, _, stderr := execute("validate", "--config", config)
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

func TestValidateAfterFlags(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("--config", "rules.yml", "validate")
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

func TestValidateAccepts(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("validate", "--config", "rules.yml")
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

const relative = `rules:
  - id: resource
    trigger: (*./resource.Resource).Open
    satisfiers: [(*./resource.Resource).Close]
`

func TestAnalyzeResolvesRelativeNames(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, stdout, stderr := execute("--config", write(t, relative), "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	if !strings.Contains(stdout, leak) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
	}
}

func TestValidateResolvesRelativeNames(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project", "leak"))
	code, _, stderr := execute("validate", "--config", write(t, relative))
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

func TestValidateRefusesRelativeNamesOutsideAModule(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, stderr := execute("validate", "--config", write(t, relative))
	if code != driver.ExitFailed {
		t.Fatalf("exit = %d, want %d", code, driver.ExitFailed)
	}
	if !strings.Contains(stderr, "relative to the module") {
		t.Errorf("stderr = %q, want it to say the name is relative to the module", stderr)
	}
}

func TestValidateAcceptsFailures(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute(
		"validate",
		"--config",
		write(t, relative+"failures: [./resource.Wrap]\n"),
	)
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, driver.ExitClean, stderr)
	}
}

func TestValidateRefusesFailures(t *testing.T) {
	tests := []struct {
		defect  string
		failure string
		want    string
	}{
		{
			defect:  "misspelled function",
			failure: "./resource.Wrpa",
			want:    "failures: example.com/project/resource.Wrpa: no such function or method",
		},
		{
			defect:  "no error returned",
			failure: "(*./resource.Resource).Open",
			want:    "failures: (example.com/project/resource.Resource).Open: a failure does not return",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, relative+"failures: ["+test.failure+"]\n")
			code, _, stderr := execute("validate", "--config", config)
			if code != driver.ExitFailed {
				t.Fatalf("exit = %d, want %d", code, driver.ExitFailed)
			}
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, test.want)
			}
		})
	}
}

func TestValidateRefuses(t *testing.T) {
	tests := []struct {
		defect string
		rule   string
		want   string
	}{
		{
			defect: "misspelled method",
			rule: `trigger: (*example.com/project/resource.Resource).Opne
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "no such function or method",
		},
		{
			defect: "missing package",
			rule: `trigger: example.com/project/missing.Open
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "no such function or method",
		},
		{
			defect: "slot out of the signature",
			rule: `trigger: {name: (*example.com/project/resource.Resource).Open, slot: argument 3}
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "slot is not in the signature",
		},
		{
			defect: "receiver of a function",
			rule: `trigger: {name: example.com/project/resource.Open, slot: receiver}
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "slot is not in the signature",
		},
		{
			defect: "no type in common",
			rule: `trigger: (*example.com/project/resource.Resource).Open
    satisfiers: [(*example.com/project/resource.Cache).Put]`,
			want: "no type links the trigger to every satisfier",
		},
		{
			defect: "ambiguous slot",
			rule: `trigger: (*example.com/project/resource.Cache).Swap
    satisfiers: [(*example.com/project/resource.Cache).Put]`,
			want: "more than one slot",
		},
		{
			defect: "no such implementation",
			rule: `trigger: (example.com/project/resource.Opener).Open
    satisfiers: [(example.com/project/resource.Opener).Close]
    implementations: [example.com/project/resource.Missing]`,
			want: "implementation example.com/project/resource.Missing: no such type",
		},
		{
			defect: "implementation in a missing package",
			rule: `trigger: (example.com/project/resource.Opener).Open
    satisfiers: [(example.com/project/resource.Opener).Close]
    implementations: [example.com/project/missing.Store]`,
			want: "implementation example.com/project/missing.Store: no such type",
		},
		{
			defect: "implementation of no interface",
			rule: `trigger: (example.com/project/resource.Opener).Open
    satisfiers: [(example.com/project/resource.Opener).Close]
    implementations: [example.com/project/resource.Cache]`,
			want: "implements no interface the rule names",
		},
		{
			defect: "implementation with a method of another receiver",
			rule: `trigger: (example.com/project/resource.Pool).Acquire
    satisfiers: [(example.com/project/resource.Recycler).Release]
    implementations: [example.com/project/resource.SQLPool]`,
			want: "has a method of the rule on a receiver it does not implement",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, "rules:\n  - id: broken\n    "+test.rule+"\n")
			code, _, stderr := execute("validate", "--config", config)
			if code != driver.ExitFailed {
				t.Fatalf("exit = %d, want %d", code, driver.ExitFailed)
			}
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, test.want)
			}
		})
	}
}

func execute(arguments ...string) (code int, stdout, stderr string) {
	var output, errors bytes.Buffer
	code = command.Run(arguments, &output, &errors)
	return code, output.String(), errors.String()
}

// reported is the base name of each file with a finding in stdout, sorted.
func reported(stdout string) []string {
	var files []string
	for line := range strings.Lines(strings.TrimSpace(stdout)) {
		path, _, _ := strings.Cut(line, ":")
		files = append(files, filepath.Base(path))
	}
	slices.Sort(files)
	return files
}

func built(version string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: version}}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
