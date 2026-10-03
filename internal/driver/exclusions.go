package driver

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
)

var (
	errConditions = errors.New(
		"at least 2 of (text, source, path[-except], linters) should be set",
	)
	errPathAndExcept = errors.New("path and path-except should not be set at the same time")
	errWarnUnused    = errors.New("linters.exclusions.warn-unused is neither true nor false")
	errRelativeTo    = errors.New("unknown relative path mode")
	errPreset        = errors.New("invalid preset")
)

// presets are the exclusion presets of golangci-lint. Every rule of theirs is
// for another linter, and drops no finding of this module's.
var presets = []string{"comments", "std-error-handling", "common-false-positives", "legacy"}

// exclusions is what linters.exclusions of a golangci-lint configuration says
// of the findings it drops, but for the generated files, which Generated
// reads: those in a file one of paths matches, or none of pathsExcept does,
// and those a rule matches (ExclusionPaths and ExclusionRules,
// pkg/result/processors). Under warnUnused, a rule that dropped none is
// warned of.
type exclusions struct {
	paths       []*regexp.Regexp
	pathsExcept []*regexp.Regexp
	rules       []exclusion
	warnUnused  bool
}

// exclusionsOf reads linters.exclusions of golangci, a golangci-lint
// configuration, as golangci-lint does: weakly typed, a text for a list split
// at its commas, and refused where golangci-lint refuses it. The presets are
// only checked. The command's own file, nil here, excludes nothing.
func exclusionsOf(golangci map[string]any) (exclusions, error) {
	linters, _ := golangci["linters"].(map[string]any)
	section, _ := linters["exclusions"].(map[string]any)
	var found exclusions
	var err error
	if found.warnUnused, err = warnUnusedOf(section["warn-unused"]); err != nil {
		return exclusions{}, err
	}
	if err := checkPresets(section["presets"]); err != nil {
		return exclusions{}, err
	}
	if found.paths, err = patternsOf("paths", section["paths"]); err != nil {
		return exclusions{}, err
	}
	if found.pathsExcept, err = patternsOf("paths-except", section["paths-except"]); err != nil {
		return exclusions{}, err
	}
	if found.rules, err = rulesOf(section["rules"]); err != nil {
		return exclusions{}, err
	}
	return found, nil
}

// excludes reports whether the paths of e drop a finding in the file at
// relative, its path from the base: one of paths matches it, or pathsExcept
// is written and none of them does.
func (e exclusions) excludes(relative string) bool {
	matches := func(pattern *regexp.Regexp) bool { return pattern.MatchString(relative) }
	if slices.ContainsFunc(e.paths, matches) {
		return true
	}
	return len(e.pathsExcept) > 0 && !slices.ContainsFunc(e.pathsExcept, matches)
}

// unmatched is found but the findings of linter a rule of e matches. It warns
// of a line a rule cannot read, which that rule then does not match, and,
// under warnUnused, of each rule for linter alone that matched none: a rule
// that may match the findings of another linter cannot be judged from the
// findings of one.
func (e exclusions) unmatched(found []finding, linter string, warn func(error)) []finding {
	used := make([]bool, len(e.rules))
	lines := make(sourceLines)
	var kept []finding
	for _, current := range found {
		index := slices.IndexFunc(e.rules, func(rule exclusion) bool {
			return rule.matches(current, linter, lines, warn)
		})
		if index < 0 {
			kept = append(kept, current)
			continue
		}
		used[index] = true
	}
	for index, rule := range e.rules {
		if e.warnUnused && !used[index] && slices.Equal(rule.linters, []string{linter}) {
			warn(fmt.Errorf("skipped 0 issues by rules: [%s]", rule))
		}
	}
	return kept
}

// exclusion is a rule of linters.exclusions: it drops a finding every
// condition it writes matches (baseRule, pkg/result/processors/base_rule.go).
type exclusion struct {
	linters    []string
	path       *regexp.Regexp
	pathExcept *regexp.Regexp
	text       *regexp.Regexp
	source     *regexp.Regexp
}

// matches reports whether every condition of e matches current, a finding of
// linter, in the order golangci-lint reads them, the line of the file last. A
// line that cannot be read is warned of, and does not match.
func (e exclusion) matches(
	current finding,
	linter string,
	lines sourceLines,
	warn func(error),
) bool {
	switch {
	case e.text != nil && !e.text.MatchString(current.message),
		e.path != nil && !e.path.MatchString(current.relative),
		e.pathExcept != nil && e.pathExcept.MatchString(current.relative),
		len(e.linters) > 0 && !slices.Contains(e.linters, linter):
		return false
	case e.source == nil:
		return true
	}
	line, err := lines.line(current.position.Filename, current.position.Line)
	if err != nil {
		warn(err)
		return false
	}
	return e.source.MatchString(line)
}

// String is e as golangci-lint names a rule when it warns of it.
func (e exclusion) String() string {
	var parts []string
	for _, condition := range []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{name: "Text", pattern: e.text},
		{name: "Source", pattern: e.source},
		{name: "Path", pattern: e.path},
		{name: "Path Except", pattern: e.pathExcept},
	} {
		if condition.pattern != nil {
			parts = append(parts, fmt.Sprintf("%s: %q", condition.name, condition.pattern))
		}
	}
	if len(e.linters) > 0 {
		parts = append(parts, fmt.Sprintf("Linters: %q", strings.Join(e.linters, ", ")))
	}
	return strings.Join(parts, ", ")
}

// sourceLines is the lines of each file a rule reads, read once.
type sourceLines map[string][]string

// line is the line of the file at path a source condition reads: the first
// when it is 0, as golangci-lint reads it, without a carriage return.
func (s sourceLines) line(path string, number int) (string, error) {
	if _, read := s[path]; !read {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to get line %s:%d: %w", path, number, err)
		}
		s[path] = strings.Split(string(content), "\n")
	}
	number = max(number, 1)
	if number > len(s[path]) {
		return "", fmt.Errorf("failed to get line %s:%d: the file has %d", path, number, len(s[path]))
	}
	return strings.Trim(s[path][number-1], "\r"), nil
}

// rulesOf reads linters.exclusions.rules: a list of maps, or a map alone, as
// a weakly typed decoder lifts it into a list of one. The keys of a rule are
// read in any case, and two that fold alike are refused. A rule is refused
// where golangci-lint refuses it (BaseRule.Validate, pkg/config/base_rule.go).
func rulesOf(value any) ([]exclusion, error) {
	var written []any
	switch typed := value.(type) {
	case nil:
	case []any:
		written = typed
	case map[string]any:
		if len(typed) > 0 {
			written = []any{typed}
		}
	default:
		written = []any{value}
	}
	rules := make([]exclusion, 0, len(written))
	for index, element := range written {
		rule, err := ruleOf(index, element)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// ruleOf reads the rule at index of linters.exclusions.rules.
func ruleOf(index int, element any) (exclusion, error) {
	name := fmt.Sprintf("linters.exclusions.rules[%d]", index)
	written, isMap := element.(map[string]any)
	if !isMap {
		return exclusion{}, fmt.Errorf("%s is not a map", name)
	}
	written, err := folded(written)
	if err != nil {
		return exclusion{}, fmt.Errorf("%s: %w", name, err)
	}
	var rule exclusion
	var isList bool
	if rule.linters, isList = listOf(written["linters"]); !isList {
		return exclusion{}, fmt.Errorf("%s.linters is not a list of texts", name)
	}
	texts := make(map[string]string)
	for _, key := range []string{"path", "path-except", "text", "source"} {
		value, isText := text(written[key])
		if written[key] != nil && !isText {
			return exclusion{}, fmt.Errorf("%s.%s is not a text", name, key)
		}
		texts[key] = value
	}
	for _, target := range []struct {
		key     string
		pattern **regexp.Regexp
	}{
		{key: "path", pattern: &rule.path},
		{key: "path-except", pattern: &rule.pathExcept},
		{key: "text", pattern: &rule.text},
		{key: "source", pattern: &rule.source},
	} {
		if texts[target.key] == "" {
			continue
		}
		if *target.pattern, err = regexp.Compile(texts[target.key]); err != nil {
			return exclusion{}, fmt.Errorf("error in exclude rule #%d: invalid %s regex: %w",
				index, target.key, err)
		}
	}
	return rule, validRule(index, rule)
}

// validRule refuses a rule that writes both path and path-except, or fewer
// than two conditions, a path and a path-except counting as one.
func validRule(index int, rule exclusion) error {
	if rule.path != nil && rule.pathExcept != nil {
		return fmt.Errorf("error in exclude rule #%d: %w", index, errPathAndExcept)
	}
	conditions := 0
	for _, written := range []bool{
		len(rule.linters) > 0,
		rule.path != nil || rule.pathExcept != nil,
		rule.text != nil,
		rule.source != nil,
	} {
		if written {
			conditions++
		}
	}
	if conditions < 2 {
		return fmt.Errorf("error in exclude rule #%d: %w", index, errConditions)
	}
	return nil
}

// patternsOf compiles linters.exclusions.<key>, a list of regular expressions.
func patternsOf(key string, value any) ([]*regexp.Regexp, error) {
	written, isList := listOf(value)
	if !isList {
		return nil, fmt.Errorf("linters.exclusions.%s is not a list of texts", key)
	}
	patterns := make([]*regexp.Regexp, 0, len(written))
	for _, expression := range written {
		pattern, err := regexp.Compile(expression)
		if err != nil {
			return nil, fmt.Errorf("linters.exclusions.%s: can't compile regexp %q: %w",
				key, expression, err)
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// checkPresets refuses linters.exclusions.presets when it names a preset
// golangci-lint does not have.
func checkPresets(value any) error {
	written, isList := listOf(value)
	if !isList {
		return errors.New("linters.exclusions.presets is not a list of texts")
	}
	for _, preset := range written {
		if !slices.Contains(presets, preset) {
			return fmt.Errorf("%w: %s", errPreset, preset)
		}
	}
	return nil
}

// warnUnusedOf reads linters.exclusions.warn-unused as golangci-lint decodes
// it, weakly typed: text as strconv.ParseBool reads it, with the empty text
// false, a number as whether it is not zero, and a key left empty as false.
func warnUnusedOf(value any) (bool, error) {
	switch typed := value.(type) {
	case nil:
		return false, nil
	case bool:
		return typed, nil
	case int:
		return typed != 0, nil
	case int64:
		return typed != 0, nil
	case uint64:
		return typed != 0, nil
	case float64:
		return typed != 0, nil
	case string:
		if typed == "" {
			return false, nil
		}
		parsed, err := strconv.ParseBool(typed)
		if err != nil {
			return false, errWarnUnused
		}
		return parsed, nil
	}
	return false, errWarnUnused
}

// listOf reads a list of texts as golangci-lint decodes it, weakly typed: a
// text split at its commas, with the empty text no element; a list of values
// read as text, an empty element as the empty text; an empty map, or a key
// left empty, as no element; and another single value as a list of one.
func listOf(value any) ([]string, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, true
	case string:
		if typed == "" {
			return nil, true
		}
		return strings.Split(typed, ","), true
	case map[string]any:
		return nil, len(typed) == 0
	case []any:
		texts := make([]string, 0, len(typed))
		for _, element := range typed {
			single, isText := text(element)
			if element != nil && !isText {
				return nil, false
			}
			texts = append(texts, single)
		}
		return texts, true
	}
	single, isText := text(value)
	return []string{single}, isText
}

// relativeTo is a mode of run.relative-path-mode: what the paths of
// linters.exclusions are read from (GetBasePath, pkg/fsutils/basepath.go).
type relativeTo string

const (
	relativeToModule     relativeTo = "gomod"
	relativeToRepository relativeTo = "gitroot"
	relativeToConfig     relativeTo = "cfg"
	relativeToDirectory  relativeTo = "wd"
)

// relativeOf reads run.relative-path-mode, a text, weakly typed: the
// configuration's directory when the key is left empty or out.
func relativeOf(value any) (relativeTo, error) {
	mode, isText := text(value)
	if value != nil && !isText {
		return "", fmt.Errorf("%w: %v", errRelativeTo, value)
	}
	switch written := relativeTo(mode); written {
	case "":
		return relativeToConfig, nil
	case relativeToModule, relativeToRepository, relativeToConfig, relativeToDirectory:
		return written, nil
	}
	return "", fmt.Errorf("%w: %s", errRelativeTo, mode)
}

// base is the directory the paths of linters.exclusions are read from, as
// golangci-lint finds it: the directory of the module go env names, the root
// of the git repository, the working directory, or configuration, the
// directory of the configuration.
func (r relativeTo) base(configuration string) (string, error) {
	switch r {
	case relativeToModule:
		module, err := output("go", "env", "GOMOD")
		if err != nil {
			return "", fmt.Errorf("get go.mod path: %w", err)
		}
		return filepath.Dir(module), nil
	case relativeToRepository:
		root, err := output("git", "rev-parse", "--show-toplevel")
		if err != nil {
			return "", fmt.Errorf("get git root: %w", err)
		}
		return root, nil
	case relativeToDirectory:
		return os.Getwd()
	}
	return configuration, nil
}

// output is what the command name prints with arguments, without the spaces
// around it.
func output(name string, arguments ...string) (string, error) {
	printed, err := exec.Command(name, arguments...).Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(printed)), nil
}
