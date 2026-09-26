//go:build golangci

package e2e_test

import (
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
	code, stdout, stderr := golangci(
		t,
		"run",
		"-c",
		golangciConfig(t, settingsOf(t, rules), nil),
		"./...",
	)
	if code != golangciIssues {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
	}
	compare(t, findings(t, stdout, " (paircheck)"))
}

func TestGolangciRefuses(t *testing.T) {
	for _, current := range refusals {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			code, _, stderr := golangci(
				t,
				"run",
				"-c",
				golangciConfig(t, settingsOf(t, current.file), nil),
				"./...",
			)
			// golangci-lint quotes the error inside its log line.
			unquoted := strings.ReplaceAll(stderr, `\"`, `"`)
			if code != golangciFailed || !refused(unquoted, current) {
				t.Errorf(
					"exit = %d, stderr = %q, want %d and %q",
					code,
					stderr,
					golangciFailed,
					current.want,
				)
			}
		})
	}
}

func TestGolangciWithoutSettings(t *testing.T) {
	code, stdout, stderr := golangci(t, "run", "-c", golangciConfig(t, nil, nil), "./...")
	if code != 0 || stdout != "" {
		t.Errorf("exit = %d, stdout = %q, want 0 and no issue; stderr: %s", code, stdout, stderr)
	}
}

// TestGolangciWithoutTests leaves the test files out under run.tests: false,
// as the command does with the same configuration.
func TestGolangciWithoutTests(t *testing.T) {
	config := golangciConfig(t, settingsOf(t, rules), map[string]any{"tests": false})
	code, stdout, stderr := golangci(t, "run", "-c", config, "./...")
	if code != golangciIssues {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, golangciIssues, stderr)
	}
	compareWith(t, findings(t, stdout, " (paircheck)"), outsideTests(t))
}

// TestGolangciSkipsTestMain runs a rule only the generated test main could
// bind.
func TestGolangciSkipsTestMain(t *testing.T) {
	code, stdout, stderr := golangci(
		t,
		"run",
		"-c",
		golangciConfig(t, settingsOf(t, testMain), nil),
		"./...",
	)
	if code != 0 || stdout != "" {
		t.Errorf("exit = %d, stdout = %q, want 0 and no issue; stderr: %s", code, stdout, stderr)
	}
}

func TestGolangciVerifiesExample(t *testing.T) {
	code, _, stderr := golangci(t, "config", "verify", "-c", absolute(t, "../golangci.example.yml"))
	if code != 0 {
		t.Errorf("exit = %d, want 0; stderr: %s", code, stderr)
	}
}

// golangci runs golangci-lint with the plugins in the project, with a cache of
// its own.
func golangci(t *testing.T, arguments ...string) (code int, stdout, stderr string) {
	t.Helper()
	binary := os.Getenv(golangciVariable)
	if binary == "" {
		t.Fatalf("%s is not set: run make e2e", golangciVariable)
	}
	cache := "GOLANGCI_LINT_CACHE=" + t.TempDir()
	return execute(t, []string{cache}, binary, arguments...)
}
