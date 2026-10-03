// Package silenced declares names //nolint directives silence, and names they
// leave reported.
package silenced

var cfgLine = 1 //nolint:defcheck

var cfgEvery = 2 //nolint

var cfgAll = 3 //nolint:all

var cfgOther = 4 //nolint:paircheck

var cfgSpaced = 5 // nolint:defcheck

var cfgListed = 6 //nolint:errcheck,DefCheck // a reason

var cfgBlock = 7 /* nolint:defcheck */

//nolint:defcheck
func cfgFunction() {
	cfgInner := 8
	_ = cfgInner
}

// a comment of the same group
//
//nolint:defcheck
var cfgGrouped = 9

var cfgAbove = 10 //nolint:defcheck
var cfgBelow = 11
