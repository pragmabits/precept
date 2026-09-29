// Package fileutil does things with a file another package opened.
package fileutil

import "os"

// CloseQuietly closes the file on every path.
func CloseQuietly(f *os.File) { _ = f.Close() }

// Describe uses the file, and neither closes it nor lets it leave.
func Describe(f *os.File) string { return f.Name() }
