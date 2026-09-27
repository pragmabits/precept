//go:build golangci

package e2e_test

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// golangciVariable names golangci-lint built with this module's plugins.
// `make e2e` builds it and sets the variable; building it clones golangci-lint,
// which is why these tests sit behind a build tag.
const golangciVariable = "PRECEPT_GOLANGCI_LINT"

// golangci-lint exits 1 with issues and 3 when a linter fails.
const (
	golangciIssues = 1
	golangciFailed = 3
)

func TestGolangciReports(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			config := current.golangciConfig(t, settingsOf(t, current.rules), nil)
			code, stdout, stderr := current.golangci(t, "run", "-c", config, "./...")
			if code != golangciIssues {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
			}
			current.compare(t, current.findings(t, stdout, " ("+current.name+")"))
		})
	}
}

func TestGolangciRefuses(t *testing.T) {
	for _, current := range linters() {
		for _, rejected := range current.refused {
			t.Run(current.name+" "+filepath.Base(rejected.file), func(t *testing.T) {
				config := current.golangciConfig(t, settingsOf(t, rejected.file), nil)
				code, _, stderr := current.golangci(t, "run", "-c", config, "./...")
				// golangci-lint quotes the error inside its log line.
				unquoted := strings.ReplaceAll(stderr, `\"`, `"`)
				if code != golangciFailed || !refused(unquoted, rejected) {
					t.Errorf(
						"exit = %d, stderr = %q, want %d and %q",
						code,
						stderr,
						golangciFailed,
						rejected.want,
					)
				}
			})
		}
	}
}

func TestGolangciWithoutSettings(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			config := current.golangciConfig(t, nil, nil)
			code, stdout, stderr := current.golangci(t, "run", "-c", config, "./...")
			if code != 0 || stdout != "" {
				t.Errorf("exit = %d, stdout = %q, want 0 and no issue; stderr: %s", code, stdout, stderr)
			}
		})
	}
}

// TestGolangciFollowsRunTests reads each value of run.tests the command's
// TestCommandFollowsRunTests reads, with the same outcome.
func TestGolangciFollowsRunTests(t *testing.T) {
	for _, test := range runTestsValues {
		t.Run(test.name, func(t *testing.T) {
			run := map[string]any{cmp.Or(test.key, "tests"): test.value}
			config := paircheck.golangciConfig(t, settingsOf(t, rules), run)
			code, stdout, stderr := paircheck.golangci(t, "run", "-c", config, "./...")
			if code != golangciIssues {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, " (paircheck)"),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
}

// TestGolangciFlagWinsOverRunTests writes --tests on the command line against
// run.tests, as TestCommandFlagWinsOverRunTests does, with the same outcome.
func TestGolangciFlagWinsOverRunTests(t *testing.T) {
	for _, test := range flagsOverRunTests {
		t.Run(test.flag, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), map[string]any{"tests": test.run})
			code, stdout, stderr := paircheck.golangci(t, "run", "-c", config, test.flag, "./...")
			if code != golangciIssues {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, " (paircheck)"),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
}

// TestGolangciSkipsAKeyAFlagReplaces writes the flags of
// TestCommandSkipsAKeyAFlagReplaces, with the same outcome.
func TestGolangciSkipsAKeyAFlagReplaces(t *testing.T) {
	for _, test := range flagsOverInvalidKeys {
		t.Run(test.name, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), test.run)
			code, stdout, stderr := paircheck.golangci(t, "run", "-c", config, test.flag, "./...")
			if code != golangciIssues {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, " (paircheck)"),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
}

// TestGolangciFollowsBuildTags reads each run.build-tags and --build-tags
// TestCommandFollowsBuildTags writes, with the same outcome.
func TestGolangciFollowsBuildTags(t *testing.T) {
	for _, test := range buildTags {
		t.Run(test.name, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), withBuildTags(test.run))
			arguments := append([]string{"run", "-c", config}, test.flags...)
			code, stdout, stderr := paircheck.golangci(t, append(arguments, "./...")...)
			if code != golangciIssues {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, " (paircheck)"),
				paircheck.expected(t, true, test.tagged, generatedStrict),
			)
		})
	}
}

// TestGolangciSkipsTestMain runs a rule only the generated test main could
// bind.
func TestGolangciSkipsTestMain(t *testing.T) {
	code, stdout, stderr := paircheck.golangci(
		t,
		"run",
		"-c",
		paircheck.golangciConfig(t, settingsOf(t, testMain), nil),
		"./...",
	)
	if code != 0 || stdout != "" {
		t.Errorf("exit = %d, stdout = %q, want 0 and no issue; stderr: %s", code, stdout, stderr)
	}
}

func TestGolangciVerifiesExample(t *testing.T) {
	code, _, stderr := paircheck.golangci(
		t,
		"config",
		"verify",
		"-c",
		absolute(t, "../golangci.example.yml"),
	)
	if code != 0 {
		t.Errorf("exit = %d, want 0; stderr: %s", code, stderr)
	}
}

// golangci runs golangci-lint with the plugins in the project, with a cache of
// its own.
func (l linter) golangci(t *testing.T, arguments ...string) (code int, stdout, stderr string) {
	t.Helper()
	binary := os.Getenv(golangciVariable)
	if binary == "" {
		t.Fatalf("%s is not set: run make e2e", golangciVariable)
	}
	cache := "GOLANGCI_LINT_CACHE=" + t.TempDir()
	return l.execute(t, []string{cache}, binary, arguments...)
}

// TestGolangciFollowsGenerated writes each mode of
// linters.exclusions.generated TestCommandFollowsGenerated writes, with the
// same outcome.
func TestGolangciFollowsGenerated(t *testing.T) {
	for _, current := range linters() {
		for _, mode := range generatedModes {
			t.Run(current.name+" "+mode, func(t *testing.T) {
				config := current.golangciGenerated(t, settingsOf(t, current.rules), mode)
				code, stdout, stderr := current.golangci(t, "run", "-c", config, "./...")
				if code != golangciIssues {
					t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
				}
				suffix := " (" + current.name + ")"
				compareWith(t, current.findings(t, stdout, suffix), current.expected(t, true, false, mode))
			})
		}
	}
}
