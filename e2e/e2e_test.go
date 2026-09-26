// Package e2e_test checks the paircheck command, the module plugin and
// golangci-lint against one project, whose source says in `// want` comments
// what each rule reports on each line. A diagnostic no comment expects fails,
// and so does a comment no diagnostic meets.
package e2e_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const (
	project = "testdata/project"
	rules   = "testdata/rules.yml"
	empty   = "testdata/empty.yml"
)

// refusal is a configuration the analysis refuses, with the error it gives
// and a text that error must not carry.
type refusal struct {
	file   string
	want   string
	absent string
}

// typeParameterHint ends the error of a rule whose type parameter sits in a
// slot the rule did not name.
const typeParameterHint = "a type parameter links only through a slot written on each side"

// refused reports whether output carries the error of current, and not the
// text it must not carry.
func refused(output string, current refusal) bool {
	return strings.Contains(output, current.want) &&
		(current.absent == "" || !strings.Contains(output, current.absent))
}

var refusals = []refusal{
	{
		file: "testdata/refused/fd-ambiguous.yml",
		want: `rule "fd": trigger example.com/project/fdio.Open: ` +
			"more than one slot has the type that links the trigger to the satisfiers",
	},
	{
		file: "testdata/refused/context-only.yml",
		want: `rule "scope": no type links the trigger to every satisfier`,
	},
	{
		file: "testdata/refused/context-one-side.yml",
		want: `rule "scope": no type links the trigger to every satisfier`,
	},
	{
		file: "testdata/refused/type-parameter.yml",
		want: `rule "hold": no type links the trigger to every satisfier: ` +
			"a type parameter links only through a slot written on each side",
	},
	{
		file: "testdata/refused/type-parameter-one-side.yml",
		want: `rule "hold": no type links the trigger to every satisfier: ` +
			"a type parameter links only through a slot written on each side",
	},
	{
		file: "testdata/refused/slice-without-slots.yml",
		want: `rule "all": no type links the trigger to every satisfier: ` +
			"a type parameter links only through a slot written on each side",
	},
	{
		file:   "testdata/refused/crossed.yml",
		want:   `rule "crossed": no type links the trigger to every satisfier`,
		absent: typeParameterHint,
	},
	{
		file:   "testdata/refused/shapes.yml",
		want:   `rule "shapes": no type links the trigger to every satisfier`,
		absent: typeParameterHint,
	},
	{
		file: "testdata/refused/slot-outside.yml",
		want: `rule "pin": trigger: example.com/project/pin.Pin: slot is not in the signature`,
	},
}

var (
	wantComment = regexp.MustCompile(`// want (.*)$`)
	literal     = regexp.MustCompile("`[^`]*`" + `|"(?:[^"\\]|\\.)*"`)
	position    = regexp.MustCompile(`^(.+?):(\d+):\d+: (.+)$`)
)

// command is the paircheck binary TestMain builds from this module.
var command string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests builds the command once for every test, and removes it after.
func runTests(m *testing.M) int {
	directory, err := os.MkdirTemp("", "precept-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(directory)
	command = filepath.Join(directory, "paircheck")
	build := exec.Command("go", "build", "-o", command, "github.com/pragmabits/precept/cmd/paircheck")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build the command: %v\n%s", err, output)
		return 1
	}
	return m.Run()
}

// diagnostic is one finding at a line of a file of the project.
type diagnostic struct {
	file    string
	line    int
	message string
}

// expectation is one `// want` pattern at a line of the project.
type expectation struct {
	file    string
	line    int
	pattern *regexp.Regexp
}

// compare reports every diagnostic no expectation matches, and every
// expectation no diagnostic met. One diagnostic meets one expectation.
func compare(t *testing.T, got []diagnostic) {
	t.Helper()
	pending := expectations(t)
	for _, finding := range got {
		index := slices.IndexFunc(pending, func(want expectation) bool {
			return want.file == finding.file && want.line == finding.line &&
				want.pattern.MatchString(finding.message)
		})
		if index < 0 {
			t.Errorf("unexpected diagnostic %s:%d: %s", finding.file, finding.line, finding.message)
			continue
		}
		pending = slices.Delete(pending, index, index+1)
	}
	for _, want := range pending {
		t.Errorf("%s:%d: no diagnostic matched %q", want.file, want.line, want.pattern)
	}
}

// expectations reads the `// want` comments of every file of the project.
func expectations(t *testing.T) []expectation {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(project, "*", "*.go"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	var found []expectation
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		file := relative(t, absolute(t, path))
		for index, text := range strings.Split(string(content), "\n") {
			match := wantComment.FindStringSubmatch(text)
			if match == nil {
				continue
			}
			for _, quoted := range literal.FindAllString(match[1], -1) {
				pattern, err := strconv.Unquote(quoted)
				if err != nil {
					t.Fatalf("%s:%d: %v", file, index+1, err)
				}
				found = append(found, expectation{
					file:    file,
					line:    index + 1,
					pattern: regexp.MustCompile(pattern),
				})
			}
		}
	}
	return found
}

// findings parses output made only of file:line:column: message lines, with
// suffix cut from each message.
func findings(t *testing.T, output, suffix string) []diagnostic {
	t.Helper()
	var found []diagnostic
	for _, text := range strings.Split(strings.TrimSpace(output), "\n") {
		if text == "" {
			continue
		}
		match := position.FindStringSubmatch(text)
		if match == nil {
			t.Errorf("output line is not a diagnostic: %q", text)
			continue
		}
		line, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("Atoi: %v", err)
		}
		found = append(found, diagnostic{
			file:    relative(t, match[1]),
			line:    line,
			message: strings.TrimSuffix(match[3], suffix),
		})
	}
	return found
}

// relative is path from the root of the project, in slashes. A relative path
// is taken as already relative to the project, as golangci-lint prints it.
func relative(t *testing.T, path string) string {
	t.Helper()
	root := absolute(t, project)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	inside, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	return filepath.ToSlash(inside)
}

func absolute(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return resolved
}

// execute runs name in the project with environment added to the current
// one, and returns its exit code and output.
func execute(
	t *testing.T,
	environment []string,
	name string,
	arguments ...string,
) (code int, stdout, stderr string) {
	t.Helper()
	var output, failures bytes.Buffer
	process := exec.Command(name, arguments...)
	process.Dir = project
	process.Env = append(os.Environ(), environment...)
	process.Stdout = &output
	process.Stderr = &failures
	err := process.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("run %s: %v", name, err)
	}
	return process.ProcessState.ExitCode(), output.String(), failures.String()
}

// settingsOf reads a rules file into what golangci-lint hands a module plugin:
// the decoded YAML, before any type is known.
func settingsOf(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var settings map[string]any
	if err := yaml.Unmarshal(content, &settings); err != nil {
		t.Fatalf("Unmarshal %s: %v", path, err)
	}
	return settings
}
