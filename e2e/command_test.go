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
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			config := absolute(t, current.rules)
			code, stdout, stderr := current.execute(t, nil, current.command, "-c", config, "./...")
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			current.compare(t, current.findings(t, stdout, ""))
		})
	}
}

// TestCommandWithoutTests leaves the test files out, and reports the rest of
// the project as without the flag.
func TestCommandWithoutTests(t *testing.T) {
	code, stdout, stderr := paircheck.execute(
		t,
		nil,
		paircheck.command,
		"-c",
		absolute(t, rules),
		"--tests=false",
		"./...",
	)
	if code != exitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
	}
	compareWith(
		t,
		paircheck.findings(t, stdout, ""),
		paircheck.expected(t, false, false, generatedStrict),
	)
}

// TestCommandFollowsRunTests reads run.tests of a golangci-lint configuration
// as golangci-lint does, and analyzes the test files when it is not written.
func TestCommandFollowsRunTests(t *testing.T) {
	for _, test := range runTestsValues {
		t.Run(test.name, func(t *testing.T) {
			run := map[string]any{cmp.Or(test.key, "tests"): test.value}
			config := paircheck.golangciConfig(t, settingsOf(t, rules), run)
			code, stdout, stderr := paircheck.execute(t, nil, paircheck.command, "-c", config, "./...")
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, ""),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
	t.Run("unset", func(t *testing.T) {
		config := paircheck.golangciConfig(t, settingsOf(t, rules), nil)
		code, stdout, stderr := paircheck.execute(t, nil, paircheck.command, "-c", config, "./...")
		if code != exitFindings {
			t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
		}
		paircheck.compare(t, paircheck.findings(t, stdout, ""))
	})
}

// TestCommandFlagWinsOverRunTests writes --tests on the command line against
// run.tests, and the flag decides, as it does in golangci-lint.
func TestCommandFlagWinsOverRunTests(t *testing.T) {
	for _, test := range flagsOverRunTests {
		t.Run(test.flag, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), map[string]any{"tests": test.run})
			code, stdout, stderr := paircheck.execute(
				t,
				nil,
				paircheck.command,
				"-c",
				config,
				test.flag,
				"./...",
			)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, ""),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
}

// TestCommandSkipsAKeyAFlagReplaces writes a flag over a key the file writes
// wrongly, and the file's key is not read.
func TestCommandSkipsAKeyAFlagReplaces(t *testing.T) {
	for _, test := range flagsOverInvalidKeys {
		t.Run(test.name, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), test.run)
			code, stdout, stderr := paircheck.execute(
				t,
				nil,
				paircheck.command,
				"-c",
				config,
				test.flag,
				"./...",
			)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, ""),
				paircheck.expected(t, test.analyzed, false, generatedStrict),
			)
		})
	}
}

// TestCommandFollowsBuildTags reads run.build-tags of a golangci-lint
// configuration as golangci-lint does, and adds --build-tags to it.
func TestCommandFollowsBuildTags(t *testing.T) {
	for _, test := range buildTags {
		t.Run(test.name, func(t *testing.T) {
			config := paircheck.golangciConfig(t, settingsOf(t, rules), withBuildTags(test.run))
			arguments := append([]string{"-c", config}, test.flags...)
			code, stdout, stderr := paircheck.execute(
				t,
				nil,
				paircheck.command,
				append(arguments, "./...")...)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			compareWith(
				t,
				paircheck.findings(t, stdout, ""),
				paircheck.expected(t, true, test.tagged, generatedStrict),
			)
		})
	}
}

// TestCommandSkipsTestMain runs a rule only the generated test main could
// bind.
func TestCommandSkipsTestMain(t *testing.T) {
	code, stdout, stderr := paircheck.execute(
		t,
		nil,
		paircheck.command,
		"-c",
		absolute(t, testMain),
		"./...",
	)
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
	for _, current := range linters() {
		t.Run(current.name, func(t *testing.T) {
			config := absolute(t, empty)
			code, stdout, stderr := current.execute(t, nil, current.command, "-c", config, "./...")
			if code != exitClean || stdout != "" || stderr != "" {
				t.Errorf(
					"exit = %d, stdout = %q, stderr = %q, want %d and no output",
					code,
					stdout,
					stderr,
					exitClean,
				)
			}
		})
	}
}

func TestCommandValidates(t *testing.T) {
	for _, path := range []string{rules, "../paircheck.example.yml", "../golangci.example.yml"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			code, _, stderr := paircheck.execute(
				t,
				nil,
				paircheck.command,
				"validate",
				"-c",
				absolute(t, path),
			)
			if code != exitClean {
				t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
			}
		})
	}
}

func TestCommandRefuses(t *testing.T) {
	for _, current := range paircheck.refused {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			config := absolute(t, current.file)
			code, stdout, stderr := paircheck.execute(t, nil, paircheck.command, "-c", config, "./...")
			if code != exitFailed {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFailed, stderr)
			}
			if strings.Count(stderr, current.want) != 1 || !refused(stderr, current) || stdout != "" {
				t.Errorf("stdout = %q, stderr = %q, want %q once on stderr", stdout, stderr, current.want)
			}
			code, _, stderr = paircheck.execute(t, nil, paircheck.command, "validate", "-c", config)
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
	code, stdout, stderr := paircheck.execute(t, nil, paircheck.command, "-v")
	if code != exitClean {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
	if !regexp.MustCompile(`^(v\S+|\(devel\))\n$`).MatchString(stdout) {
		t.Errorf("stdout = %q, want one version", stdout)
	}
}

// TestCommandFollowsGenerated reads linters.exclusions.generated of a
// golangci-lint configuration, and drops the findings in generated files as
// golangci-lint does, for each analyzer.
func TestCommandFollowsGenerated(t *testing.T) {
	for _, current := range linters() {
		for _, mode := range generatedModes {
			t.Run(current.name+" "+mode, func(t *testing.T) {
				config := current.golangciGenerated(t, settingsOf(t, current.rules), mode)
				code, stdout, stderr := current.execute(t, nil, current.command, "-c", config, "./...")
				if code != exitFindings {
					t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
				}
				compareWith(t, current.findings(t, stdout, ""), current.expected(t, true, false, mode))
			})
		}
	}
}

func TestCommandDefcheckRefuses(t *testing.T) {
	for _, current := range defcheck.refused {
		t.Run(filepath.Base(current.file), func(t *testing.T) {
			config := absolute(t, current.file)
			code, stdout, stderr := defcheck.execute(t, nil, defcheck.command, "-c", config, "./...")
			if code != exitFailed || stdout != "" || strings.Count(stderr, current.want) != 1 {
				t.Errorf(
					"exit = %d, stdout = %q, stderr = %q, want %d and %q once on stderr",
					code,
					stdout,
					stderr,
					exitFailed,
					current.want,
				)
			}
		})
	}
}

// TestCommandDefcheckAcceptsExamples runs defcheck with each example of the
// repository's root, which the command accepts.
func TestCommandDefcheckAcceptsExamples(t *testing.T) {
	for _, path := range []string{"../defcheck.example.yml", "../golangci.example.yml"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			config := absolute(t, path)
			code, _, stderr := defcheck.execute(t, nil, defcheck.command, "-c", config, "./...")
			if code == exitFailed || stderr != "" {
				t.Errorf("exit = %d, stderr = %q, want the example accepted", code, stderr)
			}
		})
	}
}
