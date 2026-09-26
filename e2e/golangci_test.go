//go:build golangci

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
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
	code, stdout, stderr := golangci(t, "run", "-c", golangciConfig(t, settingsOf(t, rules)), "./...")
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
				golangciConfig(t, settingsOf(t, current.file)),
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
	code, stdout, stderr := golangci(t, "run", "-c", golangciConfig(t, nil), "./...")
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

// golangciConfig writes a golangci-lint configuration running only paircheck
// as a module plugin, with settings, or with none when settings is nil. Paths
// are printed relative to the project, one line per issue, with no cap on the
// count.
func golangciConfig(t *testing.T, settings map[string]any) string {
	t.Helper()
	plugin := map[string]any{"type": "module"}
	if settings != nil {
		plugin["settings"] = settings
	}
	config := map[string]any{
		"version": "2",
		"run":     map[string]any{"relative-path-mode": "wd"},
		"linters": map[string]any{
			"default": "none",
			"enable":  []string{"paircheck"},
			"settings": map[string]any{
				"custom": map[string]any{
					"paircheck": plugin,
				},
			},
		},
		"issues": map[string]any{
			"max-issues-per-linter": 0,
			"max-same-issues":       0,
			"uniq-by-line":          false,
		},
		"output": map[string]any{
			"formats": map[string]any{
				"text": map[string]any{
					"path":               "stdout",
					"print-issued-lines": false,
					"colors":             false,
				},
			},
			"show-stats": false,
		},
	}
	content, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	written := filepath.Join(t.TempDir(), "golangci.yml")
	if err := os.WriteFile(written, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return written
}
