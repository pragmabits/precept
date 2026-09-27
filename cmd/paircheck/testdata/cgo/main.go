// Command cgo leaks a file in a file that imports C: go/packages hands the
// analysis the file cgo rewrites, in the build cache, with line directives to
// this one.
package main

// int two() { return 2; }
import "C"

import "os"

func main() {
	file, err := os.Open("cgo")
	if err != nil {
		return
	}
	_ = file.Name()
	_ = C.two()
}
