package driver

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/analysis/checker"
)

// finding is a diagnostic where golangci-lint places it, and the path of its
// file from the base the paths of linters.exclusions are read from, once the
// filter has read it.
type finding struct {
	position token.Position
	end      token.Position
	message  string
	relative string
}

// same reports whether f is other where golangci-lint places it, with the same
// message, whatever the filter has read of either.
func (f finding) same(other finding) bool {
	return f.position == other.position && f.end == other.end && f.message == other.message
}

// filter is what drops a finding of linter before it is printed, as the
// processors of golangci-lint drop it, in their order (NewRunner,
// pkg/lint/runner.go): one whose path from base the paths of exclusions drop,
// one in a file generated says is generated, one a rule of exclusions
// matches, and one a //nolint directive silences.
type filter struct {
	linter     string
	base       string
	generated  Generated
	exclusions exclusions
}

// kept is found but the findings f drops. It warns of a finding whose path
// cannot be read from the base, which it drops, as golangci-lint does
// (PathRelativity, pkg/result/processors/path_relativity.go), and of what the
// exclusions warn of. When a file cannot be read to tell whether it is
// generated, golangci-lint warns and drops no finding in a generated file
// (processIssues, pkg/lint/runner.go), and so does kept.
func (f filter) kept(found []finding, warn func(error)) []finding {
	var located []finding
	for _, current := range found {
		relative, err := filepath.Rel(f.base, current.position.Filename)
		if err != nil {
			warn(err)
			continue
		}
		current.relative = relative
		if !f.exclusions.excludes(relative) {
			located = append(located, current)
		}
	}
	if kept, err := f.generated.kept(located); err != nil {
		warn(fmt.Errorf("%w: no finding in a generated file is dropped", err))
	} else {
		located = kept
	}
	return unsilenced(f.exclusions.unmatched(located, f.linter, warn), f.linter)
}

// report is the diagnostics of the roots of graph as golangci-lint prints
// them, in the order of the roots, each once, and but those filter drops. A
// diagnostic outside a Go file is dropped (InvalidIssue,
// pkg/result/processors/invalid_issue.go), which drops the files cgo writes
// in the build cache, all named without .go.
func report(graph *checker.Graph, filter filter, warn func(error)) []finding {
	return filter.kept(placedFindings(graph), warn)
}

// placedFindings is the diagnostics of the roots of graph in the files
// golangci-lint places them in, each once, as the checker prints them once,
// and but those outside a Go file.
func placedFindings(graph *checker.Graph) []finding {
	var found []finding
	seen := make(map[finding]bool)
	for _, root := range graph.Roots {
		files := root.Package.Fset
		for _, diagnostic := range root.Diagnostics {
			current := finding{
				position: placed(files, root.Package.Syntax, diagnostic.Pos),
				end:      placed(files, root.Package.Syntax, diagnostic.End),
				message:  diagnostic.Message,
			}
			if filepath.Ext(current.position.Filename) != ".go" || seen[current] {
				continue
			}
			seen[current] = true
			found = append(found, current)
		}
	}
	return found
}

// write prints found to stdout, one line each, as the checker prints a
// diagnostic.
func write(stdout io.Writer, found []finding) error {
	var buffer bytes.Buffer
	for _, current := range found {
		fmt.Fprintf(&buffer, "%s: %s\n", current.position, current.message)
	}
	_, err := stdout.Write(buffer.Bytes())
	return err
}

// placed is where golangci-lint places a diagnostic at position in syntax,
// the files of a package in files (GetFilePositionFor,
// pkg/goanalysis/position.go): where a line directive maps it, when that is a
// Go file, as cgo maps the file it rewrites in the build cache to the one it
// was written from, and otherwise where it is, as after a directive to a
// template. A directive that maps the package clause of a Go file too leaves
// the diagnostics of the file in it (FilenameUnadjuster,
// pkg/result/processors/filename_unadjuster.go). golangci-lint looks that
// clause up by the name the directive maps to, and so also moves into the file
// the diagnostics of the Go file of that name, at positions that are not
// theirs; placed reads only the diagnostic's own file.
func placed(files *token.FileSet, syntax []*ast.File, position token.Pos) token.Position {
	mapped := files.PositionFor(position, true)
	compiled := files.PositionFor(position, false)
	if filepath.Ext(mapped.Filename) != ".go" {
		return compiled
	}
	if mapped.Filename != compiled.Filename && filepath.Ext(compiled.Filename) == ".go" &&
		clauseMappedTo(files, syntax, position, mapped.Filename) {
		return compiled
	}
	return mapped
}

// clauseMappedTo reports whether a line directive maps the package clause of
// the file of syntax that holds position to the file named name.
func clauseMappedTo(
	files *token.FileSet,
	syntax []*ast.File,
	position token.Pos,
	name string,
) bool {
	index := slices.IndexFunc(syntax, func(file *ast.File) bool {
		return file.FileStart <= position && position <= file.FileEnd
	})
	if index < 0 {
		return false
	}
	return files.PositionFor(syntax[index].Package, true).Filename == name
}
