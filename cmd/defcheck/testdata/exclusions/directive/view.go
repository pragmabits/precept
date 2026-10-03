// Package directive declares names after a line directive to a template,
// which is not a Go file: a finding is placed in this file, at its own line,
// and a //nolint directive on that line silences it.
package directive

//line view.tmpl:20
var cfgView = 1 //nolint:defcheck

var cfgShown = 2
