package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const leak = "leak.go:6:2: [resource] Open requires Close on r before function exit"

func TestAnalyzeReports(t *testing.T) {
	for _, config := range []string{"rules.yml", "golangci-native.yml", "golangci-plugin.yml"} {
		t.Run(config, func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute("-config", config, "./...")
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
		{"-c", "rules.yml"},
		{"--config", "rules.yml"},
		{"--config=rules.yml"},
		{"-config", "rules.yml"},
	}
	for _, spelling := range spellings {
		t.Run(strings.Join(spelling, " "), func(t *testing.T) {
			t.Chdir(filepath.Join("testdata", "project"))
			code, stdout, stderr := execute(append(spelling, "./...")...)
			if code != exitFindings {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, exitFindings, stderr)
			}
			if !strings.Contains(stdout, leak) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, leak)
			}
		})
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
	if !strings.Contains(stderr, "-c, --config") {
		t.Errorf("usage = %q, want it to name -c, --config", stderr)
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
	code, _, stderr := execute("-config", config, "./...")
	if code != exitFailed {
		t.Fatalf("exit = %d, want %d", code, exitFailed)
	}
	if !strings.Contains(stderr, "transfer") {
		t.Errorf("stderr = %q, want it to name transfer", stderr)
	}
}

func TestRequiresConfig(t *testing.T) {
	if code, _, _ := execute("./..."); code != exitFailed {
		t.Errorf("exit = %d, want %d", code, exitFailed)
	}
}

func TestValidateAccepts(t *testing.T) {
	t.Chdir(filepath.Join("testdata", "project"))
	code, _, stderr := execute("validate", "-config", "rules.yml")
	if code != exitClean {
		t.Errorf("exit = %d, want %d; stderr: %s", code, exitClean, stderr)
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
			code, _, stderr := execute("validate", "-config", config)
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

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
