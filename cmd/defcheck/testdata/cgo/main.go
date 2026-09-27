// Command cgo declares a name in a file that imports C: go/packages hands the
// analysis the file cgo rewrites, in the build cache, with line directives to
// this one, and the files cgo generates, which declare names such as
// _Cgo_ptr.
package main

// int two() { return 2; }
import "C"

var cfg = int(C.two())

func main() { _ = cfg }
