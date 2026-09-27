// Package template leaks a file after a line directive, which a generator
// writes to say the code below came from a template: the finding is printed
// in this file, where the code is, as golangci-lint prints it.
package template

import "os"

// View returns with the file open.
func View(path string) error {
//line view.tmpl:5
	file, err := os.Open(path) // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		return err
	}
	_, err = file.Stat()
	return err
}
