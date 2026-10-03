// Package testfuncs declares, outside its test files, a function named as go
// test names one it runs, which go test does not run.
package testfuncs

import "testing"

func TestCfgOutside(t *testing.T) {} // want `"TestCfgOutside" is forbidden by pattern`
