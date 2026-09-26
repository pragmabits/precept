package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

const leak = "leak.go:6:2: [resource] Open requires Close on r before function exit"

func TestAnalyzeReports(t *testing.T) {
	for _, config := range []string{"rules.yml", "golangci-native.yml", "golangci-plugin.yml"} {
		t.Run(config, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("--config", config, "./...")
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			if !strings.Contains(stdout, leak) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
			}
		})
	}
}

func TestConfigSpellings(t *testing.T) {
	spellings := [][]string{
		{"-c", "rules.yml", "./..."},
		{"--config", "rules.yml", "./..."},
		{"--config=rules.yml", "./..."},
		{"./...", "-c", "rules.yml"},
		{"./leak", "--config", "rules.yml", "./resource"},
		{"-c", "rules.yml", "--", "./..."},
	}
	for _, spelling := range spellings {
		t.Run(strings.Join(spelling, " "), func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute(spelling...)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			if !strings.Contains(stdout, leak) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
			}
		})
	}
}

func TestUnknownFlag(t *testing.T) {
	code, _, stderr := execute("--verbose", "./...")
	if code != exitFailed {
		t.Errorf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(stderr, "--verbose") {
		t.Errorf("stderr = %q, want it to name --verbose", stderr)
	}
}

func TestValidateTakesTheShortFlag(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	if code, _, stderr := execute("validate", "-c", "rules.yml"); code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
}

func TestHelp(t *testing.T) {
	code, _, stderr := execute("-h")
	if code != exitClean {
		t.Errorf("exit = %d, want %d", code, exitClean)
	}
	for _, flag := range []string{"-c, --config", "--tests", "-v, --version"} {
		if !strings.Contains(stderr, flag) {
			t.Errorf("usage = %q, want it to name %s", stderr, flag)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	for _, spelling := range []string{"-v", "--version"} {
		t.Run(spelling, func(t *testing.T) {
			code, stdout, stderr := execute(spelling)
			if code != exitClean {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
			}
			info, _ := debug.ReadBuildInfo()
			if want := version(info) + "\n"; stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		build string
		info  *debug.BuildInfo
		want  string
	}{
		{build: "module version", info: built("v0.1.0+dirty"), want: "v0.1.0+dirty"},
		{build: "no module version", info: built(""), want: "(devel)"},
		{build: "no build information", info: nil, want: "(devel)"},
	}
	for _, test := range tests {
		t.Run(test.build, func(t *testing.T) {
			if got := version(test.info); got != test.want {
				t.Errorf("version = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnalyzeRefusesInvalidConfig(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: resource
    trigger: (*example.com/project/resource.Resource).Open
    satisfiers: [(*example.com/project/resource.Resource).Close]
    transfer: true
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(stderr, "transfer") {
		t.Errorf("stderr = %q, want it to name transfer", stderr)
	}
}

func TestAnalyzeRefusesUnboundRule(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `rules:
  - id: swap
    trigger: (*example.com/project/resource.Cache).Swap
    satisfiers: [(*example.com/project/resource.Cache).Put]
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFailed, stderr)
	}
	want := `rule "swap": more than one slot`
	if strings.Count(stderr, want) != 1 {
		t.Errorf("stderr = %q, want %q once", stderr, want)
	}
}

func TestAnalyzePrintsLoadErrorsOnce(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "broken"))
	code, _, stderr := execute("--config", write(t, "rules: []\n"), "./...")
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFailed, stderr)
	}
	want := `value.go:5:27: cannot use "value"`
	if strings.Count(stderr, want) != 1 {
		t.Errorf("stderr = %q, want %q once", stderr, want)
	}
}

func TestAnalyzeRefusesRunTests(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	config := write(t, `run:
  tests: sometimes
linters:
  settings:
    paircheck:
      rules: []
`)
	code, _, stderr := execute("--config", config, "./...")
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFailed, stderr)
	}
	if !strings.Contains(stderr, "run.tests") {
		t.Errorf("stderr = %q, want it to name run.tests", stderr)
	}
}

func TestRequiresConfig(t *testing.T) {
	if code, _, _ := execute("./..."); code != exitFailed {
		t.Errorf("exit = %d, want %d", code, exitFailed)
	}
}

func TestValidateAfterFlags(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("--config", "rules.yml", "validate")
	if code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
}

func TestValidateAccepts(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("validate", "--config", "rules.yml")
	if code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
}

const relative = `rules:
  - id: resource
    trigger: (*./resource.Resource).Open
    satisfiers: [(*./resource.Resource).Close]
`

func TestAnalyzeResolvesRelativeNames(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, stdout, stderr := execute("--config", write(t, relative), "./...")
	if code != exitFindings {
		t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
	}
	if !strings.Contains(stdout, leak) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
	}
}

func TestValidateResolvesRelativeNames(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project", "leak"))
	code, _, stderr := execute("validate", "--config", write(t, relative))
	if code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
}

func TestValidateRefusesRelativeNamesOutsideAModule(t *testing.T) {
	t.Chdir(t.TempDir())
	code, _, stderr := execute("validate", "--config", write(t, relative))
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(stderr, "relative to the module") {
		t.Errorf("stderr = %q, want it to say the name is relative to the module", stderr)
	}
}

func TestValidateAcceptsFailures(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute(
		"validate",
		"--config",
		write(t, relative+"failures: [./resource.Wrap]\n"),
	)
	if code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
	}
}

func TestValidateRefusesFailures(t *testing.T) {
	tests := []struct {
		defect  string
		failure string
		want    string
	}{
		{
			defect:  "misspelled function",
			failure: "./resource.Wrpa",
			want:    "failures: example.com/project/resource.Wrpa: no such function or method",
		},
		{
			defect:  "no error returned",
			failure: "(*./resource.Resource).Open",
			want:    "failures: (example.com/project/resource.Resource).Open: a failure does not return",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, relative+"failures: ["+test.failure+"]\n")
			code, _, stderr := execute("validate", "--config", config)
			if code != exitFailed {
				t.Fatalf("exit = %d, want %d", code, exitFailed)
			}
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, test.want)
			}
		})
	}
}

func TestValidateRefuses(t *testing.T) {
	tests := []struct {
		defect string
		rule   string
		want   string
	}{
		{
			defect: "misspelled method",
			rule: `trigger: (*example.com/project/resource.Resource).Opne
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "no such function or method",
		},
		{
			defect: "missing package",
			rule: `trigger: example.com/project/missing.Open
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "no such function or method",
		},
		{
			defect: "slot out of the signature",
			rule: `trigger: {name: (*example.com/project/resource.Resource).Open, slot: argument 3}
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "slot is not in the signature",
		},
		{
			defect: "receiver of a function",
			rule: `trigger: {name: example.com/project/resource.Open, slot: receiver}
    satisfiers: [(*example.com/project/resource.Resource).Close]`,
			want: "slot is not in the signature",
		},
		{
			defect: "no type in common",
			rule: `trigger: (*example.com/project/resource.Resource).Open
    satisfiers: [(*example.com/project/resource.Cache).Put]`,
			want: "no type links the trigger to every satisfier",
		},
		{
			defect: "ambiguous slot",
			rule: `trigger: (*example.com/project/resource.Cache).Swap
    satisfiers: [(*example.com/project/resource.Cache).Put]`,
			want: "more than one slot",
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			config := write(t, "rules:\n  - id: broken\n    "+test.rule+"\n")
			code, _, stderr := execute("validate", "--config", config)
			if code != exitFailed {
				t.Fatalf("exit = %d, want %d", code, exitFailed)
			}
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, test.want)
			}
		})
	}
}

func execute(arguments ...string) (code int, stdout, stderr string) {
	var output, errors bytes.Buffer
	code = run(arguments, &output, &errors)
	return code, output.String(), errors.String()
}

func built(version string) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: version}}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
