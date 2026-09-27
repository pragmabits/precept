// Package placed holds line directives to gen.go, a file of generated code:
// one in the middle of mid.go, which golangci-lint follows, and one before the
// package clause of top.go, which it does not.
package placed

var cfgBefore = 1

//line gen.go:40
var cfgMid = 2
