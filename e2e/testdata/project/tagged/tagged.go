//go:build precept

// Package tagged is built only under the precept build tag, which
// run.build-tags or --build-tags pass: without it, the package has no file to
// analyze.
package tagged

import "os"

// Leak returns with the file open.
func Leak(path string) error {
	file, err := os.Open(path) // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		return err
	}
	_, err = file.Stat()
	return err
}
