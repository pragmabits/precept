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

// The findings of the rule ^cfg in testdata/exclusions that some exclusion can
// drop.
var (
	keptFinding   = forbidden("kept/kept.go:4:5", "cfgKept")
	pathFinding   = forbidden("paths/paths.go:4:5", "cfgPath")
	textFinding   = forbidden("rules/rules.go:5:5", "cfgText")
	sourceFinding = forbidden("rules/rules.go:7:5", "cfgSource")
	otherFinding  = forbidden("silenced/silenced.go:11:5", "cfgOther")
	blockFinding  = forbidden("silenced/silenced.go:17:5", "cfgBlock")
	belowFinding  = forbidden("silenced/silenced.go:31:5", "cfgBelow")
	shownFinding  = forbidden("directive/view.go:9:5", "cfgShown")
	rulePackages  = []string{"./kept", "./paths", "./rules"}
)

// TestAnalyzeFollowsExclusionPaths drops a finding in a file whose path from
// the base linters.exclusions.paths matches, or that no paths-except matches.
// A text written for the list is split at its commas, and an element of a list
// is not.
func TestAnalyzeFollowsExclusionPaths(t *testing.T) {
	tests := []struct {
		name       string
		exclusions string
		want       []string
	}{
		{name: "paths", exclusions: "paths: [^paths/]", want: []string{keptFinding}},
		{name: "paths-except", exclusions: "paths-except: [^paths/]", want: []string{pathFinding}},
		{
			name:       "text split at commas",
			exclusions: `paths: "^none/,^paths/"`,
			want:       []string{keptFinding},
		},
		{
			name:       "element not split",
			exclusions: `paths: ["^none/,^paths/"]`,
			want:       []string{keptFinding, pathFinding},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "exclusions"))
			config := exclusionsConfig(t, "wd", test.exclusions)
			code, stdout, stderr := execute("-c", config, "./kept", "./paths")
			if code != driver.ExitFindings || stderr != "" {
				t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
			}
			if got := findings(t, stdout); !slices.Equal(got, test.want) {
				t.Errorf("findings = %q, want %q", got, test.want)
			}
		})
	}
}

// TestAnalyzeFollowsRelativePathMode reads the paths of linters.exclusions from
// the base run.relative-path-mode names: by default the directory of the
// configuration, which here is not the project's.
func TestAnalyzeFollowsRelativePathMode(t *testing.T) {
	tests := []struct {
		mode string
		path string
		want []string
	}{
		{mode: "cfg", path: "^paths/", want: []string{keptFinding, pathFinding}},
		{mode: "", path: "^paths/", want: []string{keptFinding, pathFinding}},
		{mode: "wd", path: "^paths/", want: []string{keptFinding}},
		{mode: "gomod", path: "^paths/", want: []string{keptFinding}},
		{
			mode: "gitroot",
			path: "^cmd/defcheck/testdata/exclusions/paths/",
			want: []string{keptFinding},
		},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "exclusions"))
			config := exclusionsConfig(t, test.mode, "paths: ["+test.path+"]")
			code, stdout, stderr := execute("-c", config, "./kept", "./paths")
			if code != driver.ExitFindings || stderr != "" {
				t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
			}
			if got := findings(t, stdout); !slices.Equal(got, test.want) {
				t.Errorf("findings = %q, want %q", got, test.want)
			}
		})
	}
}

// TestAnalyzeReadsPathsFromTheConfigurationDirectory runs with a configuration
// in the project, whose paths are read from where it is.
func TestAnalyzeReadsPathsFromTheConfigurationDirectory(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "exclusions"))
	code, stdout, stderr := execute("-c", "golangci.yml", "./kept", "./paths")
	if code != driver.ExitFindings || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
	}
	if got, want := findings(t, stdout), []string{keptFinding}; !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzeRefusesGitRootOutsideGit refuses paths read from the root of a
// git repository when the command runs outside of one.
func TestAnalyzeRefusesGitRootOutsideGit(t *testing.T) {
	project := t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/outside\n\ngo 1.26.0\n",
		"main.go": "package main\n\nvar cfgOutside = 1\n\nfunc main() {}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(project, name), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	t.Chdir(project)
	config := exclusionsConfig(t, "gitroot", "paths: [^none/]")
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitFailed || stdout != "" || !strings.Contains(stderr, "git") {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and git named",
			code,
			stdout,
			stderr,
			driver.ExitFailed,
		)
	}
}

// TestAnalyzeFollowsExclusionRules drops a finding every condition of an
// exclusion rule matches: its linter, the path of its file, its message, and
// the line it is on.
func TestAnalyzeFollowsExclusionRules(t *testing.T) {
	tests := []struct {
		name       string
		exclusions string
		want       []string
	}{
		{
			name:       "text and linters",
			exclusions: "rules: [{text: cfgText, linters: [defcheck]}]",
			want:       []string{keptFinding, pathFinding, sourceFinding},
		},
		{
			name:       "source and path",
			exclusions: "rules: [{source: marked, path: ^rules/}]",
			want:       []string{keptFinding, pathFinding, textFinding},
		},
		{
			name:       "path-except and text",
			exclusions: "rules: [{path-except: ^rules/, text: package-var}]",
			want:       []string{textFinding, sourceFinding},
		},
		{
			name:       "another linter",
			exclusions: "rules: [{text: cfgText, linters: [errcheck]}]",
			want:       []string{keptFinding, pathFinding, textFinding, sourceFinding},
		},
		{
			name:       "linters split at commas",
			exclusions: `rules: [{text: cfgText, linters: "errcheck,defcheck"}]`,
			want:       []string{keptFinding, pathFinding, sourceFinding},
		},
		{
			name:       "keys in any case",
			exclusions: "rules: [{Text: cfgText, LINTERS: [defcheck]}]",
			want:       []string{keptFinding, pathFinding, sourceFinding},
		},
		{
			name:       "a rule for a list",
			exclusions: "rules: {text: cfgText, linters: [defcheck]}",
			want:       []string{keptFinding, pathFinding, sourceFinding},
		},
		{
			name:       "presets",
			exclusions: "presets: [comments, std-error-handling, common-false-positives, legacy]",
			want:       []string{keptFinding, pathFinding, textFinding, sourceFinding},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "exclusions"))
			config := exclusionsConfig(t, "wd", test.exclusions)
			code, stdout, stderr := execute(append([]string{"-c", config}, rulePackages...)...)
			if code != driver.ExitFindings || stderr != "" {
				t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
			}
			if got := findings(t, stdout); !slices.Equal(got, test.want) {
				t.Errorf("findings = %q, want %q", got, test.want)
			}
		})
	}
}

// TestAnalyzeWarnsUnusedExclusions warns, under linters.exclusions.warn-unused,
// of a rule for this linter alone that dropped no finding. A path, and a rule
// that may drop the findings of another linter, are not judged: the command
// sees only its own findings.
func TestAnalyzeWarnsUnusedExclusions(t *testing.T) {
	rules := "rules: [" +
		"{text: nomatch, linters: [defcheck]}, " +
		"{text: cfgText, linters: [defcheck]}, " +
		"{text: nomatch, linters: [errcheck]}, " +
		"{text: nomatch, path: ^rules/}]"
	tests := []struct {
		warn string
		want string
	}{
		{
			warn: "true",
			want: `defcheck: skipped 0 issues by rules: [Text: "nomatch", Linters: "defcheck"]` + "\n",
		},
		{warn: "false", want: ""},
	}
	for _, test := range tests {
		t.Run(test.warn, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "exclusions"))
			config := exclusionsConfig(t, "wd", "warn-unused: "+test.warn, rules, "paths: [^none/]")
			code, stdout, stderr := execute("-c", config, "./rules")
			if code != driver.ExitFindings || stderr != test.want {
				t.Fatalf(
					"exit = %d, stderr = %q, want %d and %q",
					code,
					stderr,
					driver.ExitFindings,
					test.want,
				)
			}
			if got, want := findings(t, stdout), []string{sourceFinding}; !slices.Equal(got, want) {
				t.Errorf("findings = %q, want %q", got, want)
			}
		})
	}
}

func TestAnalyzeRefusesInvalidExclusions(t *testing.T) {
	tests := []struct {
		defect     string
		exclusions string
		want       string
	}{
		{
			defect:     "one condition",
			exclusions: "rules: [{text: cfgText}]",
			want:       "error in exclude rule #0: at least 2 of (text, source, path[-except], linters)",
		},
		{
			defect:     "no linter",
			exclusions: "rules: [{text: cfgText, linters: []}]",
			want:       "error in exclude rule #0: at least 2 of (text, source, path[-except], linters)",
		},
		{
			defect:     "path and path-except",
			exclusions: "rules: [{text: cfgText, path: a, path-except: b}]",
			want:       "error in exclude rule #0: path and path-except should not be set at the same time",
		},
		{
			defect:     "invalid text",
			exclusions: `rules: [{text: "(cfg", linters: [defcheck]}]`,
			want:       "error in exclude rule #0: invalid text regex",
		},
		{
			defect:     "invalid source",
			exclusions: `rules: [{source: "(cfg", linters: [defcheck]}]`,
			want:       "error in exclude rule #0: invalid source regex",
		},
		{
			defect:     "invalid path",
			exclusions: `rules: [{path: "(cfg", linters: [defcheck]}]`,
			want:       "error in exclude rule #0: invalid path regex",
		},
		{
			defect:     "invalid path-except",
			exclusions: `rules: [{path-except: "(cfg", linters: [defcheck]}]`,
			want:       "error in exclude rule #0: invalid path-except regex",
		},
		{
			defect:     "rule not a map",
			exclusions: "rules: [cfgText]",
			want:       "linters.exclusions.rules[0] is not a map",
		},
		{
			defect:     "text not a text",
			exclusions: "rules: [{text: [cfgText], linters: [defcheck]}]",
			want:       "linters.exclusions.rules[0].text is not a text",
		},
		{
			defect:     "keys that fold alike",
			exclusions: "rules: [{text: a, Text: b, linters: [defcheck]}]",
			want:       "keys differ only in case: Text and text",
		},
		{
			defect:     "invalid paths",
			exclusions: `paths: ["(paths"]`,
			want:       `linters.exclusions.paths: can't compile regexp "(paths"`,
		},
		{
			defect:     "invalid paths-except",
			exclusions: `paths-except: ["(paths"]`,
			want:       `linters.exclusions.paths-except: can't compile regexp "(paths"`,
		},
		{
			defect:     "paths not a list",
			exclusions: "paths: {a: b}",
			want:       "linters.exclusions.paths is not a list of texts",
		},
		{
			defect:     "unknown preset",
			exclusions: "presets: [nope]",
			want:       "invalid preset: nope",
		},
		{
			defect:     "warn-unused not a boolean",
			exclusions: `warn-unused: "yes"`,
			want:       "linters.exclusions.warn-unused is neither true nor false",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "exclusions"))
			config := exclusionsConfig(t, "wd", test.exclusions)
			code, stdout, stderr := execute("-c", config, "./kept")
			if code != driver.ExitFailed || stdout != "" || !strings.Contains(stderr, test.want) {
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

func TestAnalyzeRefusesUnknownRelativePathMode(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "exclusions"))
	config := exclusionsConfig(t, "GoMod", "paths: [^paths/]")
	code, stdout, stderr := execute("-c", config, "./kept")
	want := "unknown relative path mode: GoMod"
	if code != driver.ExitFailed || stdout != "" || !strings.Contains(stderr, want) {
		t.Errorf(
			"exit = %d, stdout = %q, stderr = %q, want %d and %q",
			code,
			stdout,
			stderr,
			driver.ExitFailed,
			want,
		)
	}
}

// TestAnalyzeFollowsNolint drops a finding a //nolint directive silences for
// every linter or for this one: on its line, or on the line above a
// declaration in the same column, which it silences whole. A directive is
// read at the lines of the file it is written in, where the command places a
// finding after a line directive to a template.
func TestAnalyzeFollowsNolint(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "exclusions"))
	config := exclusionsConfig(t, "wd")
	code, stdout, stderr := execute("-c", config, "./silenced", "./directive")
	if code != driver.ExitFindings || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q, want %d and none", code, stderr, driver.ExitFindings)
	}
	want := []string{shownFinding, otherFinding, blockFinding, belowFinding}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestAnalyzeReportsTestsGoTestRefuses reports a function named as go test
// names one it runs, whose signature go test refuses: the command analyzes the
// package all the same, and go test runs none of them. One it runs is left
// out.
func TestAnalyzeReportsTestsGoTestRefuses(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "signature"))
	config := write(t, "rules:\n  - pattern: (?i)cfg\n    kinds: [function]\n")
	code, stdout, stderr := execute("-c", config, "./...")
	if code != driver.ExitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, driver.ExitFindings, stderr)
	}
	forbidden := func(position, name string) string {
		return position + `: declaration name "` + name +
			`" is forbidden by pattern "(?i)cfg" for function`
	}
	want := []string{
		forbidden("signature_test.go:11:6", "BenchmarkCfgOther"),
		forbidden("signature_test.go:13:6", "FuzzCfgGeneric"),
		forbidden("signature_test.go:7:6", "TestCfgArgument"),
		forbidden("signature_test.go:9:6", "TestCfgResult"),
	}
	if got := findings(t, stdout); !slices.Equal(got, want) {
		t.Errorf("findings = %q, want %q", got, want)
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

// exclusionsConfig writes a golangci-lint configuration with the rule ^cfg,
// with run.relative-path-mode as mode, or unwritten when mode is empty, and
// with each line of exclusions under linters.exclusions.
func exclusionsConfig(t *testing.T, mode string, exclusions ...string) string {
	t.Helper()
	var content strings.Builder
	content.WriteString("version: \"2\"\n")
	if mode != "" {
		content.WriteString("run:\n  relative-path-mode: " + mode + "\n")
	}
	content.WriteString(
		"linters:\n  settings:\n    defcheck:\n      rules:\n        - pattern: ^cfg\n",
	)
	if len(exclusions) > 0 {
		content.WriteString("  exclusions:\n")
	}
	for _, line := range exclusions {
		content.WriteString("    " + line + "\n")
	}
	return write(t, content.String())
}

// forbidden is the finding of the rule ^cfg on a package variable named name,
// at position.
func forbidden(position, name string) string {
	return position + `: declaration name "` + name + `" is forbidden by pattern "^cfg" for package-var`
}

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
