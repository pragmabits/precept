package e2e_test

import (
	"cmp"
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
	compareWith(t, findings(t, stdout, ""), expected(t, false, false))
}

// TestCommandFollowsRunTests reads run.tests of a golangci-lint configuration
// as golangci-lint does, and analyzes the test files when it is not written.
func TestCommandFollowsRunTests(t *testing.T) {
	for _, test := range runTestsValues {
		t.Run(test.name, func(t *testing.T) {
			run := map[string]any{cmp.Or(test.key, "tests"): test.value}
			config := golangciConfig(t, settingsOf(t, rules), run)
			code, stdout, stderr := execute(t, nil, command, "-c", config, "./...")
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(t, findings(t, stdout, ""), expected(t, test.analyzed, false))
		})
	}
	t.Run("unset", func(t *testing.T) {
		config := golangciConfig(t, settingsOf(t, rules), nil)
		code, stdout, stderr := execute(t, nil, command, "-c", config, "./...")
		if code != exitFindings {
			t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
		}
		compare(t, findings(t, stdout, ""))
	})
}

// TestCommandFlagWinsOverRunTests writes --tests on the command line against
// run.tests, and the flag decides, as it does in golangci-lint.
func TestCommandFlagWinsOverRunTests(t *testing.T) {
	for _, test := range flagsOverRunTests {
		t.Run(test.flag, func(t *testing.T) {
			config := golangciConfig(t, settingsOf(t, rules), map[string]any{"tests": test.run})
			code, stdout, stderr := execute(t, nil, command, "-c", config, test.flag, "./...")
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(t, findings(t, stdout, ""), expected(t, test.analyzed, false))
		})
	}
}

// TestCommandFollowsBuildTags reads run.build-tags of a golangci-lint
// configuration as golangci-lint does, and adds --build-tags to it.
func TestCommandFollowsBuildTags(t *testing.T) {
	for _, test := range buildTags {
		t.Run(test.name, func(t *testing.T) {
			config := golangciConfig(t, settingsOf(t, rules), withBuildTags(test.run))
			arguments := append([]string{"-c", config}, test.flags...)
			code, stdout, stderr := execute(t, nil, command, append(arguments, "./...")...)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(t, findings(t, stdout, ""), expected(t, true, test.tagged))
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
