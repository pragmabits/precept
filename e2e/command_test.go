package e2e_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The exit codes of the command.
const (
	exitClean    = 0
	exitFailed   = 1
	exitFindings = 3
)

func TestCommandReports(t *testing.T) {
	code, stdout, stderr := execute(t, nil, command, "-c", absolute(t, rules), "./...")
	if code != exitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
	}
	compare(t, findings(t, stdout, ""))
}

func TestCommandWithoutRules(t *testing.T) {
	code, stdout, stderr := execute(t, nil, command, "-c", absolute(t, empty), "./...")
	if code != exitClean || stdout != "" || stderr != "" {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and no output",
			code,
			stdout,
			stderr,
			exitClean,
		)
	}
}

func TestCommandValidates(t *testing.T) {
	for _, path := range []string{rules, "../paircheck.example.yml", "../golangci.example.yml"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			code, _, stderr := execute(t, nil, command, "validate", "-c", absolute(t, path))
			if code != exitClean {
				t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
			}
		})
	}
}

func TestCommandRefuses(t *testing.T) {
	for _, current := range refusals {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			config := absolute(t, current.file)
			code, stdout, stderr := execute(t, nil, command, "-c", config, "./...")
			if code != exitFailed {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFailed, stderr)
			}
			if strings.Count(stderr, current.want) != 1 || !refused(stderr, current) || stdout != "" {
				t.Errorf("stdout = %q, stderr = %q, want %q once on stderr", stdout, stderr, current.want)
			}
			code, _, stderr = execute(t, nil, command, "validate", "-c", config)
			if code != exitFailed || !refused(stderr, current) {
				t.Errorf(
					"validate: exit = %d, stderr = %q, want %d and %q",
					code,
					stderr,
					exitFailed,
					current.want,
				)
			}
		})
	}
}

func TestCommandVersion(t *testing.T) {
	code, stdout, stderr := execute(t, nil, command, "-v")
	if code != exitClean {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
	if !regexp.MustCompile(`^(v\S+|\(devel\))\n$`).MatchString(stdout) {
		t.Errorf("stdout = %q, want one version", stdout)
	}
}
