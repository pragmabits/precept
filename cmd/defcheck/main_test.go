package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/pragmabits/precept/internal/driver"
)

// The findings of rules.yml in testdata/project, one per file and line.
const (
	packageVariable = `names/names.go:3:5: declaration name "cfg" is forbidden: ` +
		"avoid the cfg abbreviation"
	localVariable = `names/names.go:6:2: declaration name "cfg" is forbidden: ` +
		"avoid the cfg abbreviation"
	testVariable = `names/names_test.go:6:2: declaration name "cfg" is forbidden: ` +
		"avoid the cfg abbreviation"
	taggedVariable = `tagged/tagged.go:5:5: declaration name "cfg" is forbidden: ` +
		"avoid the cfg abbreviation"
	generatedVariable = `generated/generated.go:5:5: declaration name "cfg" is forbidden: ` +
		"avoid the cfg abbreviation"
)

func TestAnalyzeReports(t *testing.T) {
	for _, config := range []string{"rules.yml", "golangci-native.yml", "golangci-plugin.yml"} {
		t.Run(config, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("--config", config, "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			want := []string{packageVariable, localVariable, testVariable}
			if got := findings(t, stdout); !slices.Equal(got, want) {
				t.Errorf("findings = %q, want %q", got, want)
			}
		})
	}
}

// TestAnalyzeFollowsTheRunFlags writes each flag over the run section of a
// golangci-lint configuration, and the finding of the package it loads or
// leaves out comes and goes with it.
func TestAnalyzeFollowsTheRunFlags(t *testing.T) {
	tests := []struct {
		flag string
		want []string
	}{
		{flag: "--tests=false", want: []string{packageVariable, localVariable}},
		{
			flag: "--build-tags=precept",
			want: []string{packageVariable, localVariable, testVariable, taggedVariable},
		},
	}
	for _, test := range tests {
		t.Run(test.flag, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("-c", "golangci-native.yml", test.flag, "./...")
			if code != driver.ExitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
			}
			if got := findings(t, stdout); !slices.Equal(got, test.want) {
				t.Errorf("findings = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnalyzeRefusesModulesDownloadMode(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, stdout, stderr := execute("-c", "rules.yml", "--modules-download-mode=later", "./...")
	if code != driver.ExitFailed || stdout != "" ||
		!strings.Contains(stderr, "neither mod, readonly nor vendor") {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and the mode refused",
			code,
			stdout,
			stderr,
			driver.ExitFailed,
		)
	}
}

// TestAnalyzeReadsRunTests reads run.tests of a golangci-lint configuration.
func TestAnalyzeReadsRunTests(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, "run:\n  tests: false\n"+rulesSettings)
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	want := []string{packageVariable, localVariable}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzeFollowsGenerated drops the finding in a generated file, as
// golangci-lint does by default, and keeps it when
// linters.exclusions.generated is disable.
func TestAnalyzeFollowsGenerated(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, "linters:\n  exclusions:\n    generated: disable\n"+
		strings.TrimPrefix(rulesSettings, "linters:\n"))
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	want := []string{generatedVariable, packageVariable, localVariable, testVariable}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzePlacesACgoFindingInTheGoFile reports a declaration of a file
// that imports C where golangci-lint places it: in that file, not in the one
// cgo rewrites in the build cache, which is generated.
func TestAnalyzePlacesACgoFindingInTheGoFile(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "cgo"))
	config := write(t, "rules:\n  - pattern: ^cfg$\n")
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	want := []string{
		`main.go:10:5: declaration name "cfg" is forbidden by pattern "^cfg$" for package-var`,
	}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzeDropsCgoArtifacts matches the names cgo generates, whose files
// are not Go files, and reports nothing, as golangci-lint does, even when no
// generated file is dropped.
func TestAnalyzeDropsCgoArtifacts(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "cgo"))
	config := write(t, "linters:\n  exclusions:\n    generated: disable\n"+
		"  settings:\n    defcheck:\n      rules:\n        - pattern: ^_C\n")
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitClean || stdout != "" || stderr != "" {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and no output",
			code,
			stdout,
			stderr,
			driver.ExitClean,
		)
	}
}

// TestAnalyzePlacesFindingsAfterLineDirectives reads and prints a finding
// after a line directive where golangci-lint does: where the directive points,
// when that is a Go file, and otherwise where the finding is. A directive over
// the package clause, as in top.go, leaves each finding of the file in it,
// where golangci-lint also moves there those of gen.go, the file it points to.
func TestAnalyzePlacesFindingsAfterLineDirectives(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "directive"))
	config := write(t, "rules:\n  - pattern: ^cfg\n")
	code, stdout, stderr := execute("-c", config, "./placed", "./template")
	if code != driver.ExitFindings || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
	}
	want := []string{
		`placed/mid.go:6:5: declaration name "cfgBefore" is forbidden by pattern "^cfg" for package-var`,
		`placed/top.go:4:5: declaration name "cfgTop" is forbidden by pattern "^cfg" for package-var`,
		`template/view.go:5:5: declaration name "cfgBefore" is forbidden by pattern "^cfg" for package-var`,
		`template/view.go:8:5: declaration name "cfgTemplate" is forbidden by pattern "^cfg" for package-var`,
	}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzeKeepsGeneratedFilesWhenOneCannotBeRead points a line directive
// to a file that does not exist: as golangci-lint, the command warns that it
// cannot tell whether that file is generated, and drops no finding in a
// generated file.
func TestAnalyzeKeepsGeneratedFilesWhenOneCannotBeRead(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "directive"))
	config := write(t, "rules:\n  - pattern: ^cfg\n")
	code, stdout, stderr := execute("-c", config, "./missing")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	want := []string{
		`missing/missing.go:5:5: declaration name "cfgGenerated" is forbidden by pattern "^cfg" for package-var`,
		`missing/nowhere.go:10: declaration name "cfgNowhere" is forbidden by pattern "^cfg" for package-var`,
		`missing/plain.go:5:5: declaration name "cfgPlain" is forbidden by pattern "^cfg" for package-var`,
	}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
	if !strings.Contains(stderr, "no finding in a generated file is dropped") ||
		!strings.Contains(stderr, "nowhere.go") {
		t.Errorf("stderr = %q, want a warning that names nowhere.go", stderr)
	}
}

func TestAnalyzeWithoutRules(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, stdout, stderr := execute("-c", write(t, "rules: []\n"), "./...")
	if code != driver.ExitClean || stdout != "" || stderr != "" {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and no output",
			code,
			stdout,
			stderr,
			driver.ExitClean,
		)
	}
}

func TestAnalyzeRefusesInvalidConfig(t *testing.T) {
	tests := []struct {
		defect  string
		content string
		want    string
	}{
		{
			defect:  "invalid pattern",
			content: "rules:\n  - pattern: (cfg\n",
			want:    "rule 0: pattern is not a regular expression",
		},
		{
			defect:  "unknown kind",
			content: "rules:\n  - pattern: cfg\n    kinds: [variable]\n",
			want:    "rule 0: kind is not function",
		},
		{
			defect:  "unknown key",
			content: "rules:\n  - pattern: cfg\n    kind: field\n",
			want:    `unknown field "kind"`,
		},
		{
			defect:  "duplicate rule",
			content: "rules:\n  - pattern: cfg\n  - pattern: cfg\n",
			want:    "rule 1: rule repeats the pattern, kinds and message of another rule",
		},
		{
			defect:  "no settings",
			content: "version: \"2\"\nlinters:\n  settings: {}\n",
			want:    "the golangci-lint configuration has no defcheck settings",
		},
		{
			defect: "both settings",
			content: "linters:\n  settings:\n    defcheck:\n      rules: []\n" +
				"    custom:\n      defcheck:\n        settings:\n          rules: []\n",
			want: "both native and module plugin defcheck settings",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("-c", write(t, test.content), "./...")
			if code != driver.ExitFailed || stdout != "" ||
				!strings.Contains(stderr, test.want) {
				t.Errorf(
					"exit = %d, stdout = %q, stderr = %q, want %d and %q",
					code,
					stdout,
					stderr,
					driver.ExitFailed,
					test.want,
				)
			}
		})
	}
}

func TestAnalyzePrintsLoadErrorsOnce(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "broken"))
	code, stdout, stderr := execute("--config", write(t, "rules: []\n"), "./...")
	if code != driver.ExitFailed || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q, want %d; stderr: %s", code, stdout, driver.ExitFailed, stderr)
	}
	want := `value.go:5:27: cannot use "value"`
	if strings.Count(stderr, want) != 1 {
		t.Errorf("stderr = %q, want %q once", stderr, want)
	}
}

func TestRequiresConfig(t *testing.T) {
	code, _, stderr := execute("./...")
	if code != driver.ExitFailed || !strings.Contains(stderr, "-c, --config is required") {
		t.Errorf("exit = %d, stderr = %q, want %d and the flag named", code, stderr, driver.ExitFailed)
	}
}

func TestRequiresPackages(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("-c", "rules.yml")
	if code != driver.ExitFailed || !strings.Contains(stderr, "no packages to analyze") {
		t.Errorf("exit = %d, stderr = %q, want %d and no packages", code, stderr, driver.ExitFailed)
	}
}

func TestUnknownFlag(t *testing.T) {
	code, _, stderr := execute("--verbose", "./...")
	if code != driver.ExitFailed || !strings.Contains(stderr, "--verbose") {
		t.Errorf("exit = %d, stderr = %q, want %d and --verbose named", code, stderr, driver.ExitFailed)
	}
}

func TestHelp(t *testing.T) {
	code, _, stderr := execute("-h")
	if code != driver.ExitClean {
		t.Errorf("exit = %d, want %d", code, driver.ExitClean)
	}
	for _, flag := range []string{
		"usage: defcheck",
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

// rulesSettings is rules.yml as the native settings of a golangci-lint
// configuration.
const rulesSettings = `linters:
  settings:
    defcheck:
      rules:
        - pattern: ^cfg$
          kinds: [package-var, local-var]
          message: avoid the cfg abbreviation
`

func execute(arguments ...string) (code int, stdout, stderr string) {
	var output, errors bytes.Buffer
	code = command.Run(arguments, &output, &errors)
	return code, output.String(), errors.String()
}

// findings is each line of stdout, from the project's root, sorted: the
// packages print in the order they load in.
func findings(t *testing.T, stdout string) []string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	var found []string
	for line := range strings.Lines(strings.TrimSpace(stdout)) {
		inside, _ := strings.CutPrefix(strings.TrimSuffix(line, "\n"), root+string(filepath.Separator))
		found = append(found, filepath.ToSlash(inside))
	}
	slices.Sort(found)
	return found
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
