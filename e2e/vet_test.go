package e2e_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// exitVetFindings is what go vet exits with when a vet tool reports.
const exitVetFindings = 1

// TestVetReports runs each command as go vet's vet tool. go vet analyzes the
// test files and drops no finding of a generated file. It prints a finding
// where a //line directive maps it, outside a Go file too: the findings of
// template/view.go, which the other drivers leave there, it prints in
// view.tmpl. Only the findings in Go files are compared.
func TestVetReports(t *testing.T) {
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			code, _, stderr := current.vet(t, absolute(t, current.rules))
			if code != exitVetFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitVetFindings, stderr)
			}
			pending := slices.DeleteFunc(
				current.expected(t, true, false, generatedDisable),
				func(want expectation) bool {
					return strings.HasPrefix(want.file, "template/")
				},
			)
			compareWith(t, current.findings(t, inGoFiles(stderr), ""), pending)
			if !strings.Contains(stderr, "view.tmpl:") {
				t.Errorf("stderr = %q, want the findings of template/view.go in view.tmpl", stderr)
			}
		})
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
