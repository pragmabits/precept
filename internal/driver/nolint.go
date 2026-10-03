package driver

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strings"
)

// directive matches the text of a comment, past its slashes and spaces, that
// is a //nolint directive (NolintFilter, pkg/result/processors/nolint_filter.go).
var directive = regexp.MustCompile(`^nolint( |:|$)`)

// silence is the lines of a file on which a //nolint directive drops the
// findings of linters, or of every linter when it names none: those of the
// comment group it is in, and, when that group ends on the line above a node
// in the group's column, those of the node too.
type silence struct {
	from    int
	to      int
	column  int
	linters []string
}

// covers reports whether s drops a finding of linter on line.
func (s silence) covers(line int, linter string) bool {
	return s.from <= line && line <= s.to &&
		(len(s.linters) == 0 || slices.Contains(s.linters, linter))
}

// unsilenced is found but the findings of linter a //nolint directive of their
// file drops, each file read once.
func unsilenced(found []finding, linter string) []finding {
	silences := make(map[string][]silence)
	var kept []finding
	for _, current := range found {
		path := current.position.Filename
		if _, read := silences[path]; !read {
			silences[path] = silencesIn(path)
		}
		silenced := slices.ContainsFunc(silences[path], func(s silence) bool {
			return s.covers(current.position.Line, linter)
		})
		if !silenced {
			kept = append(kept, current)
		}
	}
	return kept
}

// silencesIn is the silences of the //nolint directives of the Go file at
// path, at the lines of the file itself. golangci-lint reads them at the lines
// a line directive gives them, and so misses a finding it places in the file
// after a directive to a template. A file that does not parse has none, as in
// golangci-lint, where the type checker has already reported it.
func silencesIn(path string) []silence {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	var inline []silence
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if found, isDirective := silenceOf(files, group, comment.Text); isDirective {
				inline = append(inline, found)
			}
		}
	}
	if len(inline) == 0 {
		return nil
	}
	expanded := slices.Clone(inline)
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		start := files.PositionFor(node.Pos(), false)
		index := slices.IndexFunc(inline, func(s silence) bool {
			return s.to == start.Line-1 && s.column == start.Column
		})
		if index >= 0 {
			grown := inline[index]
			grown.to = max(grown.to, files.PositionFor(node.End(), false).Line)
			expanded = append(expanded, grown)
		}
		return true
	})
	return expanded
}

// silenceOf is the silence of the comment text in group, and false when the
// comment is no //nolint directive. A directive that names no linter, or names
// all, silences every linter; the names are read in lower case, up to a
// comment after the directive.
func silenceOf(files *token.FileSet, group *ast.CommentGroup, text string) (silence, bool) {
	text = strings.TrimLeft(text, "/ ")
	if !directive.MatchString(text) {
		return silence{}, false
	}
	start := files.PositionFor(group.Pos(), false)
	found := silence{
		from:   start.Line,
		to:     files.PositionFor(group.End(), false).Line,
		column: start.Column,
	}
	if strings.HasPrefix(text, "nolint:all") || !strings.HasPrefix(text, "nolint:") {
		return found, true
	}
	text, _, _ = strings.Cut(text, "//")
	for item := range strings.SplitSeq(strings.TrimPrefix(text, "nolint:"), ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "all" {
			found.linters = nil
			return found, true
		}
		found.linters = append(found.linters, name)
	}
	return found, true
}
