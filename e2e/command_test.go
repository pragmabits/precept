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

// TestCommandWithoutTests leaves the test files out, and reports the rest of
// the project as without the flag.
func TestCommandWithoutTests(t *testing.T) {
	code, stdout, stderr := execute(
		t,
		nil,
		command,
		"-c",
		absolute(t, rules),
		"--tests=false",
		"./...",
	)
	if code != exitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
	}
	compareWith(t, findings(t, stdout, ""), outsideTests(t))
}

// TestCommandFollowsRunTests reads run.tests of a golangci-lint configuration
// as golangci-lint does: --tests written on the command line wins over it.
func TestCommandFollowsRunTests(t *testing.T) {
	tests := []struct {
		name     string
		run      map[string]any
		flags    []string
		reported bool
	}{
		{name: "unset", run: nil, flags: nil, reported: true},
		{name: "false", run: map[string]any{"tests": false}, flags: nil, reported: false},
		{name: "true", run: map[string]any{"tests": true}, flags: nil, reported: true},
		{
			name:     "false under --tests",
			run:      map[string]any{"tests": false},
			flags:    []string{"--tests"},
			reported: true,
		},
		{
			name:     "true under --tests=false",
			run:      map[string]any{"tests": true},
			flags:    []string{"--tests=false"},
			reported: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := golangciConfig(t, settingsOf(t, rules), test.run)
			arguments := append([]string{"-c", config}, test.flags...)
			code, stdout, stderr := execute(t, nil, command, append(arguments, "./...")...)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			want := outsideTests(t)
			if test.reported {
				want = expectations(t)
			}
			compareWith(t, findings(t, stdout, ""), want)
		})
	}
}

// TestCommandSkipsTestMain runs a rule only the generated test main could
// bind.
func TestCommandSkipsTestMain(t *testing.T) {
	code, stdout, stderr := execute(t, nil, command, "-c", absolute(t, testMain), "./...")
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
