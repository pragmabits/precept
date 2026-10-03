package e2e_test

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

// exitVetFindings is what go vet exits with when a vet tool reports.
const exitVetFindings = 1

// TestVetReports runs each command as go vet's vet tool, which reports what
// the command reports: it drops the findings in generated files as strict
// does, and prints those of template/view.go there, where the command places
// them, and not in view.tmpl, where its //line directive maps them.
func TestVetReports(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			code, _, stderr := current.vet(t, absolute(t, current.rules))
			if code != exitVetFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitVetFindings, stderr)
			}
			if strings.Contains(stderr, "view.tmpl") {
				t.Errorf("stderr = %q, want no finding in view.tmpl", stderr)
			}
			current.compare(t, current.findings(t, inGoFiles(stderr), ""))
		})
	}
}

// TestVetFollowsGenerated writes each mode of linters.exclusions.generated
// TestCommandFollowsGenerated writes, with the same outcome.
func TestVetFollowsGenerated(t *testing.T) {
	for _, current := range linters() {
		for _, mode := range generatedModes {
			t.Run(current.name+" "+mode, func(t *testing.T) {
				document := current.vetDocument(settingsOf(t, current.rules), nil)
				linters, _ := document["linters"].(map[string]any)
				linters["exclusions"] = map[string]any{"generated": mode}
				code, _, stderr := current.vet(t, writeConfig(t, document))
				if code != exitVetFindings {
					t.Fatalf("exit = %d, want %d; stderr: %s", code, exitVetFindings, stderr)
				}
				compareWith(
					t,
					current.findings(t, inGoFiles(stderr), ""),
					current.expected(t, true, false, mode),
				)
			})
		}
	}
}

// TestVetFollowsRunTests drops the findings in the test files under run.tests
// false: go vet analyzes them, and the tool cannot leave them out of the load.
func TestVetFollowsRunTests(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			run := map[string]any{"tests": false}
			config := writeConfig(t, current.vetDocument(settingsOf(t, current.rules), run))
			code, _, stderr := current.vet(t, config)
			if code != exitVetFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitVetFindings, stderr)
			}
			compareWith(
				t,
				current.findings(t, inGoFiles(stderr), ""),
				current.expected(t, false, false, generatedStrict),
			)
		})
	}
}

// TestVetFollowsExclusions drops what the paths and rules of
// linters.exclusions and //nolint directives drop, as the command does in
// TestCommandFollowsExclusions.
func TestVetFollowsExclusions(t *testing.T) {
	current := excluded()
	code, _, stderr := current.vet(t, current.excludedConfig(t))
	if code != exitVetFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitVetFindings, stderr)
	}
	current.compare(t, current.findings(t, inGoFiles(stderr), ""))
}

// TestVetRefusesRunKeys refuses, on every run, a key of run the tool cannot
// follow: go vet builds the packages under its own -tags, and runs the tool in
// the directory of each package, which no path can be read from.
func TestVetRefusesRunKeys(t *testing.T) {
	tests := []struct {
		name string
		run  map[string]any
		want string
	}{
		{
			name: "build tags",
			run:  map[string]any{"build-tags": []any{"precept"}},
			want: "run.build-tags cannot be applied under go vet: pass -tags to go vet",
		},
		{
			name: "working directory",
			run:  map[string]any{"relative-path-mode": "wd"},
			want: "run.relative-path-mode wd cannot be followed under go vet",
		},
	}
	for _, current := range linters() {
		for _, test := range tests {
			t.Run(current.name+" "+test.name, func(t *testing.T) {
				config := writeConfig(t, current.vetDocument(settingsOf(t, current.rules), test.run))
				for run := range 2 {
					code, _, stderr := current.vet(t, config)
					if code == 0 || !strings.Contains(stderr, test.want) {
						t.Errorf(
							"run %d: exit = %d, stderr = %q, want a failure with %q",
							run+1,
							code,
							stderr,
							test.want,
						)
					}
				}
			})
		}
	}
}

// TestVetRefusesARelativeConfig refuses the rules by a relative path: go vet
// runs the tool in the directory of each package. It refuses them on every run:
// go vet keeps the facts of a run that reported an error, and not the error.
func TestVetRefusesARelativeConfig(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			for run := range 2 {
				code, _, stderr := current.vet(t, "rules.yml")
				if code == 0 || !strings.Contains(stderr, "absolute") {
					t.Errorf(
						"run %d: exit = %d, stderr = %q, want a failure asking for an absolute path",
						run+1,
						code,
						stderr,
					)
				}
			}
		})
	}
}

// TestVetRefuses fails on each configuration the linter refuses, with its
// error, on every run.
func TestVetRefuses(t *testing.T) {
	for _, current := range linters() {
		for _, refusal := range current.refused {
			t.Run(current.name+"/"+filepath.Base(refusal.file), func(t *testing.T) {
				for run := range 2 {
					code, _, stderr := current.vet(t, absolute(t, refusal.file))
					if code == 0 || !refused(stderr, refusal) {
						t.Errorf(
							"run %d: exit = %d, stderr = %q, want a failure with %q",
							run+1,
							code,
							stderr,
							refusal.want,
						)
					}
				}
			})
		}
	}
}

// TestVetSkipsTestMain passes the rule only the test main go test generates
// could bind: go vet does not analyze the test main.
func TestVetSkipsTestMain(t *testing.T) {
	code, stdout, stderr := paircheck.vet(t, absolute(t, testMain))
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want 0 and no output", code, stdout, stderr)
	}
}

// vet runs go vet over the project with the command of l as its vet tool and
// the rules of config.
func (l linter) vet(t *testing.T, config string) (code int, stdout, stderr string) {
	t.Helper()
	return l.execute(t, nil, "go", "vet", "-vettool="+l.command, "-config="+config, "./...")
}

// vetDocument is the golangci-lint configuration of golangciDocument with the
// paths of linters.exclusions read from its own directory, as go vet runs the
// tool in the directory of each package, and with run added to its run
// section.
func (l linter) vetDocument(settings, run map[string]any) map[string]any {
	section := map[string]any{"relative-path-mode": "cfg"}
	maps.Copy(section, run)
	return l.golangciDocument(settings, section)
}

// inGoFiles is output without the lines of a finding outside a Go file, and
// without the lines go vet names a package by.
func inGoFiles(output string) string {
	var kept []string
	for _, line := range strings.Split(output, "\n") {
		file, _, _ := strings.Cut(line, ":")
		if strings.HasSuffix(file, ".go") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
