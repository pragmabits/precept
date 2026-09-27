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
	"golang.org/x/tools/go/packages"
)

// finding is a diagnostic where golangci-lint places it.
type finding struct {
	position token.Position
	end      token.Position
	message  string
}

// report is the diagnostics of the roots of graph as golangci-lint prints
// them, in the order of the roots, each once. A diagnostic outside a Go file
// is dropped (InvalidIssue, pkg/result/processors/invalid_issue.go), which
// drops the files cgo writes in the build cache, all named without .go; and
// so is one in a file generated says is generated. When a file cannot be read
// to tell, golangci-lint warns and drops no finding in a generated file
// (processIssues, pkg/lint/runner.go), and so does report, through warn.
func report(graph *checker.Graph, generated Generated, warn func(error)) []finding {
	found := placedFindings(graph)
	kept, err := generated.kept(found)
	if err != nil {
		warn(err)
		return found
	}
	return kept
}

// placedFindings is the diagnostics of the roots of graph in the files
// golangci-lint places them in, each once, as the checker prints them once,
// and but those outside a Go file.
func placedFindings(graph *checker.Graph) []finding {
	var found []finding
	seen := make(map[finding]bool)
	for _, root := range graph.Roots {
		for _, diagnostic := range root.Diagnostics {
			current := finding{
				position: placed(root.Package, diagnostic.Pos),
				end:      placed(root.Package, diagnostic.End),
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

// placed is where golangci-lint places a diagnostic at position in loaded
// (GetFilePositionFor, pkg/goanalysis/position.go): where a line directive
// maps it, when that is a Go file, as cgo maps the file it rewrites in the
// build cache to the one it was written from, and otherwise where it is, as
// after a directive to a template. A directive that maps the package clause
// of a Go file too leaves the diagnostics of the file in it
// (FilenameUnadjuster, pkg/result/processors/filename_unadjuster.go).
// golangci-lint looks that clause up by the name the directive maps to, and
// so also moves into the file the diagnostics of the Go file of that name, at
// positions that are not theirs; placed reads only the diagnostic's own file.
func placed(loaded *packages.Package, position token.Pos) token.Position {
	mapped := loaded.Fset.PositionFor(position, true)
	compiled := loaded.Fset.PositionFor(position, false)
	if filepath.Ext(mapped.Filename) != ".go" {
		return compiled
	}
	if mapped.Filename != compiled.Filename && filepath.Ext(compiled.Filename) == ".go" &&
		clauseMappedTo(loaded, position, mapped.Filename) {
		return compiled
	}
	return mapped
}

// clauseMappedTo reports whether a line directive maps the package clause of
// the file of loaded that holds position to the file named name.
func clauseMappedTo(loaded *packages.Package, position token.Pos, name string) bool {
	index := slices.IndexFunc(loaded.Syntax, func(file *ast.File) bool {
		return file.FileStart <= position && position <= file.FileEnd
	})
	if index < 0 {
		return false
	}
	return loaded.Fset.PositionFor(loaded.Syntax[index].Package, true).Filename == name
}
