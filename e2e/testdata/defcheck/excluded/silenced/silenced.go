// Package silenced declares names //nolint directives silence, and names they
// leave reported.
package silenced

var cfgLine = 1 //nolint:defcheck

var cfgEvery = 2 //nolint

var cfgOther = 3 //nolint:paircheck // want `^declaration name "cfgOther" is forbidden by pattern "\^cfg" for package-var$`

var cfgBlock = 4 /* nolint:defcheck */ // want `^declaration name "cfgBlock" is forbidden by pattern "\^cfg" for package-var$`

//nolint:defcheck
func cfgFunction() {
	cfgInner := 5
	_ = cfgInner
}
